package turbo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/smithy-go"
	"github.com/jmespath/go-jmespath"
)

// Query is one aws_query request. Exactly one of Action, Queries or Macro is set.
type Query struct {
	Action     string          `json:"action,omitempty"`
	Params     json.RawMessage `json:"params,omitempty"`
	Fields     string          `json:"fields,omitempty"`
	Projection string          `json:"projection,omitempty"`
	Format     string          `json:"format,omitempty"`
	Group      string          `json:"group,omitempty"`
	All        bool            `json:"all,omitempty"`
	Region     string          `json:"region,omitempty"`
	Regions    []string        `json:"regions,omitempty"`
	Queries    []Query         `json:"queries,omitempty"`
	Macro      string          `json:"macro,omitempty"`
}

var regionName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const (
	maxBatch     = 8
	maxPages     = 25
	footerBudget = 300
)

type Engine struct {
	Registry      Registry // fixed actions; consulted before the SDK-backed services
	Services      *ServiceSet
	Cache         *DeltaCache
	Mutations     *MutationGuard
	MaxBytes      int
	Timeout       time.Duration
	defaultRegion string
	reads         *readCache
}

func NewEngine(r Registry) *Engine {
	return &Engine{Registry: r, Cache: NewDeltaCache(256, 30*time.Minute), MaxBytes: 32768, Timeout: 30 * time.Second, reads: newReadCache(128, 15*time.Second)}
}

func (e *Engine) find(name string) (*Action, error) {
	if a, ok := e.Registry[name]; ok {
		return a, nil
	}
	if e.Services != nil {
		return e.Services.Resolve(name)
	}
	return nil, fmt.Errorf("unsupported action %q; use aws_discover", name)
}

// Action returns the resolved action for a name; the result is shared and cached.
func (e *Engine) Action(name string) (*Action, error) { return e.find(name) }

// DisableReadCache makes every query reach its action; used by measurements.
func (e *Engine) DisableReadCache() { e.reads = nil }

func validRegion(r string) error {
	if len(r) > 64 || !regionName.MatchString(r) {
		return fmt.Errorf("invalid AWS region")
	}
	return nil
}

// Query runs a read. It dispatches to a macro, a batch, a regional fan-out or
// a single action.
func (e *Engine) Query(ctx context.Context, session string, q Query) (string, error) {
	modes := 0
	for _, set := range []bool{q.Action != "", len(q.Queries) > 0, q.Macro != ""} {
		if set {
			modes++
		}
	}
	if modes != 1 {
		return "", fmt.Errorf("set exactly one of action, queries or macro")
	}
	switch {
	case q.Macro != "":
		return e.Diagnose(ctx, session, q.Macro, q.Params)
	case len(q.Queries) > 0:
		return e.queryBatch(ctx, session, q)
	case len(q.Regions) != 0:
		return e.queryRegions(ctx, session, q)
	}
	return e.queryOne(ctx, session, q)
}

func (e *Engine) queryOne(ctx context.Context, session string, q Query) (string, error) {
	a, err := e.find(q.Action)
	if err != nil {
		return "", err
	}
	if a.Write {
		return "", fmt.Errorf("%s changes state; use aws_mutate", a.Name)
	}
	if q.Region != "" {
		if err := validRegion(q.Region); err != nil {
			return "", err
		}
		ctx = withRegion(ctx, q.Region)
	}
	if err := validFormat(q.Format); err != nil {
		return "", err
	}
	if q.Projection != "" && q.Fields != "" {
		return "", fmt.Errorf("use fields or projection, not both")
	}
	expr := a.expression
	var order []string // column order requested through fields
	switch {
	case q.Projection != "":
		expr, err = jmespath.Compile(q.Projection)
		if err != nil {
			return "", fmt.Errorf("invalid projection: %w", err)
		}
	case q.Fields != "":
		if a.expression == nil {
			return "", fmt.Errorf("fields is unavailable for %s; use projection", a.Name)
		}
		var p string
		p, order, err = fieldsProjection(a.root, strings.Split(q.Fields, ","))
		if err != nil {
			return "", err
		}
		if expr, err = jmespath.Compile(p); err != nil {
			return "", fmt.Errorf("invalid fields: %w", err)
		}
	case expr == nil:
		expr, err = jmespath.Compile(a.Projection)
		if err != nil {
			return "", err
		}
	}
	p, err := parameters(q.Params)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	_, explicitToken := p["NextToken"]
	delta := a.Name == "logs.GetLogEvents" && !explicitToken
	region := q.Region
	if region == "" {
		region = e.defaultRegion
	}
	var cacheID string
	if !delta {
		b, _ := json.Marshal(p)
		cacheID = strings.Join([]string{region, a.Name, string(b), q.Projection, q.Fields, q.Format, q.Group, fmt.Sprint(q.All)}, "\x00")
		if text, ok := e.reads.get(cacheID); ok {
			return text, nil
		}
	}
	key := ""
	if delta {
		if _, ok := p["Limit"]; !ok {
			p["Limit"] = 200
		}
		b, _ := json.Marshal(p)
		key = cacheKey(session+"\x00"+region, b)
		unlock, err := e.Cache.lock(ctx, key)
		if err != nil {
			return "", err
		}
		defer unlock()
		if prior := e.Cache.get(key, time.Now()); prior.Token != "" {
			p["NextToken"] = prior.Token
			p["StartFromHead"] = true
		}
		if _, ok := p["StartFromHead"]; !ok {
			p["StartFromHead"] = true
		}
	}

	var rows []any
	var single any
	var lastRaw map[string]any
	var token any
	pages := 0
	for {
		args, _ := json.Marshal(p)
		if a.Validate != nil {
			if err := a.Validate(args); err != nil {
				return "", err
			}
		}
		raw, err := a.Call(ctx, args)
		if err != nil {
			return "", &callError{prefix: a.Name, msg: ShortError(err), err: err}
		}
		projected, err := expr.Search(raw)
		if err != nil {
			return "", fmt.Errorf("projection: %w", err)
		}
		pages++
		lastRaw, _ = raw.(map[string]any)
		token = nil
		if a.TokenOut != "" {
			if t := lastRaw[a.TokenOut]; t != nil && t != "" {
				token = t
			}
		}
		list, isList := projected.([]any)
		if isList {
			rows = append(rows, list...)
		} else {
			single = projected
		}
		if !q.All || delta || token == nil || !isList || pages >= maxPages || approxSize(rows) > e.MaxBytes*2 {
			break
		}
		p[a.TokenIn] = token
	}
	var result any
	switch {
	case single != nil:
		result = single
	case rows != nil:
		result = rows
	}
	if q.Group != "" {
		grouped, err := groupCounts(rows, q.Group)
		if err != nil {
			return "", err
		}
		result = grouped
	}
	rendered, err := renderOrdered(result, q.Format, e.MaxBytes-footerBudget, order)
	if err != nil {
		return "", err
	}
	text := rendered.Text
	if delta && rendered.Truncated() {
		return "", outputLimit(e.MaxBytes)
	}
	switch {
	case rendered.Truncated():
		text += fmt.Sprintf("\ntruncated: %d of %d rows; narrow params, use fields, or group", rendered.Shown, rendered.Total)
	case token != nil && !delta:
		b, _ := json.Marshal(map[string]any{a.TokenIn: token})
		text += "\nnext_params=" + string(b)
	}
	if pages > 1 {
		text += fmt.Sprintf("\npages=%d", pages)
	}
	if delta {
		text += "\ndelta=true; repeat identical params for the next page/poll"
	}
	if _, err := bounded(text, e.MaxBytes); err != nil {
		return "", err
	}
	if delta && token != nil {
		var latest float64
		if events, ok := lastRaw["Events"].([]any); ok {
			for _, event := range events {
				if row, ok := event.(map[string]any); ok {
					if ts, ok := row["Timestamp"].(float64); ok && ts > latest {
						latest = ts
					}
				}
			}
		}
		if s, ok := token.(string); ok {
			e.Cache.put(key, cursor{Token: s, Timestamp: latest, At: time.Now()}, time.Now())
		}
	}
	if !delta {
		e.reads.put(cacheID, text)
	}
	return text, nil
}

func approxSize(rows []any) int {
	if len(rows) == 0 {
		return 0
	}
	b, _ := json.Marshal(rows)
	return len(b)
}

// groupCounts replaces rows by how many rows share each value of field.
func groupCounts(rows []any, field string) (any, error) {
	counts := map[string]int{}
	for _, row := range rows {
		obj, _ := row.(map[string]any)
		v, ok := obj[field]
		if !ok {
			return nil, fmt.Errorf("group field %q is not in the result; add it to fields", field)
		}
		counts[cellText(v)]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	out := make([]any, len(keys))
	for i, k := range keys {
		out[i] = map[string]any{field: k, "Count": float64(counts[k])}
	}
	return out, nil
}

// ShortError reduces an SDK error to its AWS error code and message.
func ShortError(err error) string {
	var api smithy.APIError
	if errors.As(err, &api) {
		msg := api.ErrorMessage()
		if len(msg) > 300 {
			msg = msg[:300] + "..."
		}
		hint := ""
		switch api.ErrorCode() {
		case "ExpiredToken", "ExpiredTokenException", "RequestExpired":
			hint = " (credentials expired; refresh them, e.g. aws sso login)"
		}
		return api.ErrorCode() + ": " + msg + hint
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "failed to retrieve credentials"), strings.Contains(s, "no EC2 IMDS role found"), strings.Contains(s, "failed to refresh cached credentials"):
		return "no valid AWS credentials (set a profile/SSO login, or environment credentials)"
	case strings.Contains(s, "context deadline exceeded"):
		return "timed out; narrow the request or raise --timeout"
	}
	if len(s) > 400 {
		s = s[:400] + "..."
	}
	return s
}

type batchResult struct {
	label string
	text  string
	err   error
}

// runParallel executes work for each index with bounded concurrency.
func runParallel(ctx context.Context, n int, work func(i int) batchResult) []batchResult {
	results := make([]batchResult, n)
	limit := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case limit <- struct{}{}:
				defer func() { <-limit }()
			case <-ctx.Done():
				results[i].err = ctx.Err()
				return
			}
			results[i] = work(i)
		}()
	}
	wg.Wait()
	return results
}

func (e *Engine) joinResults(results []batchResult) (string, error) {
	var out strings.Builder
	succeeded := 0
	for _, r := range results {
		out.WriteString("### " + r.label + "\n")
		if r.err != nil {
			out.WriteString("Error: " + r.err.Error() + "\n")
		} else {
			succeeded++
			out.WriteString(r.text + "\n")
		}
	}
	if succeeded == 0 {
		return "", fmt.Errorf("all queries failed:\n%s", out.String())
	}
	if succeeded < len(results) {
		out.WriteString("partial=true\n")
	}
	return bounded(strings.TrimSuffix(out.String(), "\n"), e.MaxBytes)
}

// queryBatch runs several independent reads in parallel and labels each result.
func (e *Engine) queryBatch(ctx context.Context, session string, q Query) (string, error) {
	if len(q.Queries) > maxBatch {
		return "", fmt.Errorf("at most %d queries per call", maxBatch)
	}
	for i, sub := range q.Queries {
		if sub.Action == "" || len(sub.Queries) > 0 || sub.Macro != "" {
			return "", fmt.Errorf("queries[%d] must be a single action query", i)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	// Each result gets an equal share so one large answer cannot starve the rest.
	share := e.MaxBytes / len(q.Queries)
	results := runParallel(ctx, len(q.Queries), func(i int) batchResult {
		sub := q.Queries[i]
		label := fmt.Sprintf("%d %s", i+1, sub.Action)
		if len(sub.Regions) != 0 {
			text, err := e.withBudget(share, func(s *Engine) (string, error) { return s.queryRegions(ctx, session, sub) })
			return batchResult{label: label, text: text, err: err}
		}
		text, err := e.withBudget(share, func(s *Engine) (string, error) { return s.queryOne(ctx, session, sub) })
		return batchResult{label: label, text: text, err: err}
	})
	return e.joinResults(results)
}

// withBudget runs fn on a shallow copy of the engine with a smaller output limit.
func (e *Engine) withBudget(maxBytes int, fn func(*Engine) (string, error)) (string, error) {
	if maxBytes >= e.MaxBytes {
		return fn(e)
	}
	c := *e
	c.MaxBytes = max(maxBytes, 1024)
	return fn(&c)
}

// queryRegions runs the same read against a bounded set of regions and keeps
// the output ordered as requested, regardless of completion order.
func (e *Engine) queryRegions(ctx context.Context, session string, q Query) (string, error) {
	if q.Region != "" || len(q.Regions) > maxBatch {
		return "", fmt.Errorf("use region or up to %d regions, not both", maxBatch)
	}
	seen := map[string]bool{}
	for _, region := range q.Regions {
		if validRegion(region) != nil || seen[region] {
			return "", fmt.Errorf("regions must contain unique valid AWS regions")
		}
		seen[region] = true
	}
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	share := e.MaxBytes / len(q.Regions)
	results := runParallel(ctx, len(q.Regions), func(i int) batchResult {
		one := q
		one.Region, one.Regions = q.Regions[i], nil
		text, err := e.withBudget(share, func(s *Engine) (string, error) { return s.queryOne(ctx, session, one) })
		return batchResult{label: q.Regions[i], text: text, err: err}
	})
	return e.joinResults(results)
}

// Discover finds actions and recipes. With no search it lists services. A search
// of several words matches names containing every word. An exact action returns
// its parameters and the fields it returns.
func (e *Engine) Discover(search string) (string, error) {
	type entry struct {
		Action string `json:"action"`
		Write  bool   `json:"write,omitempty"`
	}
	type catalog struct {
		Services   []string          `json:"services,omitempty"`
		Actions    []entry           `json:"actions,omitempty"`
		Total      int               `json:"total,omitempty"`
		Action     string            `json:"action,omitempty"`
		Write      bool              `json:"write,omitempty"`
		Projection string            `json:"default_projection,omitempty"`
		Required   []string          `json:"required,omitempty"`
		Parameters map[string]string `json:"parameters,omitempty"`
		Returns    []string          `json:"returns,omitempty"`
		Macros     map[string]any    `json:"macros,omitempty"`
		Usage      string            `json:"usage,omitempty"`
	}
	const usage = "aws_query: action=service.Operation, params=exact SDK field names; fields=\"Id,State.Name\" keeps only those; all=true pages up to the output limit; group=Field counts rows; queries=[{...}] runs up to 8 reads in parallel; region/regions pick regions; macro=name runs a recipe. next_params from a result continue a page. aws_mutate previews writes; execution needs server opt-in and approval."
	search = strings.TrimSpace(search)
	c := catalog{Usage: usage}
	terms := strings.Fields(strings.ToLower(search))
	if a, err := e.find(search); err == nil && search != "" {
		c.Usage = ""
		c.Action, c.Write, c.Projection, c.Required, c.Parameters = a.Name, a.Write, a.Projection, a.Required, a.Parameters
		if e.Services != nil {
			c.Returns = e.Services.Describe(a)
		}
		b, err := json.Marshal(c)
		if err != nil {
			return "", err
		}
		return bounded(string(b), e.MaxBytes)
	}
	if len(terms) == 0 && e.Services != nil {
		c.Services = e.Services.names
	}
	const limit = 60
	if len(terms) > 0 || e.Services == nil {
		var names []string
		var writes map[string]bool
		if e.Services != nil {
			names, writes, c.Total = e.Services.Search(terms, limit)
		}
		for name, a := range e.Registry {
			match := true
			for _, t := range terms {
				match = match && strings.Contains(strings.ToLower(name), t)
			}
			if match {
				names = append(names, name)
				if a.Write {
					writes = map[string]bool{name: true}
				}
				c.Total++
			}
		}
		for _, n := range names {
			c.Actions = append(c.Actions, entry{Action: n, Write: writes[n]})
		}
		if c.Total > len(c.Actions) {
			c.Usage += fmt.Sprintf(" Showing %d of %d matches; add words to narrow.", len(c.Actions), c.Total)
		}
	}
	for name, spec := range macroCatalog() {
		if len(terms) == 0 || strings.Contains(name, strings.Join(terms, "_")) || matchesAll(name+" "+fmt.Sprint(spec), terms) {
			if c.Macros == nil {
				c.Macros = map[string]any{}
			}
			c.Macros[name] = spec
		}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return bounded(string(b), e.MaxBytes)
}

func matchesAll(s string, terms []string) bool {
	s = strings.ToLower(s)
	for _, t := range terms {
		if !strings.Contains(s, t) {
			return false
		}
	}
	return true
}

// callError carries the short message agents see and the original error for errors.Is.
type callError struct {
	prefix, msg string
	err         error
}

func (c *callError) Error() string { return c.prefix + ": " + c.msg }
func (c *callError) Unwrap() error { return c.err }
