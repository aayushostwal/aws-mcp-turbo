package turbo

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jmespath/go-jmespath"
)

type Query struct {
	Action     string          `json:"action"`
	Params     json.RawMessage `json:"params,omitempty"`
	Projection string          `json:"projection,omitempty"`
	Format     string          `json:"format,omitempty"`
	Region     string          `json:"region,omitempty"`
}

var regionName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Engine struct {
	Registry       Registry
	Cache          *DeltaCache
	Mutations      *MutationGuard
	MaxBytes       int
	Timeout        time.Duration
	defaultRegion  string
	regionRegistry func(string) Registry
}

func NewEngine(r Registry) *Engine {
	return &Engine{Registry: r, Cache: NewDeltaCache(256, 30*time.Minute), MaxBytes: 32768, Timeout: 30 * time.Second}
}

func (e *Engine) Query(ctx context.Context, session string, q Query) (string, error) {
	a, err := e.Registry.Find(q.Action)
	if err != nil {
		return "", err
	}
	if a.Write {
		return "", fmt.Errorf("state-changing action requires aws_mutate")
	}
	if q.Region != "" {
		if len(q.Region) > 64 || !regionName.MatchString(q.Region) {
			return "", fmt.Errorf("invalid AWS region")
		}
		if e.regionRegistry == nil {
			return "", fmt.Errorf("per-query region is unavailable")
		}
		a, err = e.regionRegistry(q.Region).Find(q.Action)
		if err != nil {
			return "", err
		}
	}
	if err := validFormat(q.Format); err != nil {
		return "", err
	}
	expr := a.expression
	if q.Projection != "" {
		expr, err = jmespath.Compile(q.Projection)
		if err != nil {
			return "", fmt.Errorf("invalid projection: %w", err)
		}
	}
	if expr == nil {
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
	key := ""
	if delta {
		if _, ok := p["Limit"]; !ok {
			p["Limit"] = 200
		}
		b, _ := json.Marshal(p)
		region := q.Region
		if region == "" {
			region = e.defaultRegion
		}
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
	args, _ := json.Marshal(p)
	if a.Validate != nil {
		if err := a.Validate(args); err != nil {
			return "", err
		}
	}
	raw, err := a.Call(ctx, args)
	if err != nil {
		return "", fmt.Errorf("%s: %w", a.Name, err)
	}
	projected, err := expr.Search(raw)
	if err != nil {
		return "", fmt.Errorf("projection: %w", err)
	}
	text, err := Format(projected, q.Format, e.MaxBytes)
	if err != nil {
		return "", err
	}
	obj, _ := raw.(map[string]any)
	token, _ := obj[a.TokenOut].(string)
	if token != "" && !delta {
		b, _ := json.Marshal(map[string]string{a.TokenIn: token})
		text += "\nnext_params=" + string(b)
	}
	if delta {
		text += "\ndelta=true; repeat identical params for the next page/poll"
	}
	if _, err := bounded(text, e.MaxBytes); err != nil {
		return "", err
	}
	if delta && token != "" {
		var latest float64
		if events, ok := obj["Events"].([]any); ok {
			for _, event := range events {
				if row, ok := event.(map[string]any); ok {
					if ts, ok := row["Timestamp"].(float64); ok && ts > latest {
						latest = ts
					}
				}
			}
		}
		e.Cache.put(key, cursor{Token: token, Timestamp: latest, At: time.Now()}, time.Now())
	}
	return text, nil
}

func (e *Engine) Discover(search string) (string, error) {
	type catalog struct {
		Actions []*Action      `json:"actions"`
		Macros  map[string]any `json:"macros,omitempty"`
		Usage   string         `json:"usage"`
	}
	c := catalog{Actions: e.Registry.Discover(search), Usage: "action=service.Operation; params use exact Go SDK field names. projection=JMESPath; format=markdown|tsv|json; optional region overrides the server default for this query. One AWS page per query; merge next_params into params and keep region for later pages. logs.GetLogEvents auto-polls per session/region; explicit NextToken bypasses caching. Mutation execute defaults false (local preview); execution requires server opt-in and approval hook."}
	for name, spec := range macroCatalog() {
		if strings.Contains(name, strings.ToLower(search)) || search == "" {
			if c.Macros == nil {
				c.Macros = map[string]any{}
			}
			c.Macros[name] = spec
		}
	}
	// Listing names/presets stays small; full parameter inventories are on-demand.
	if _, exact := e.Registry[search]; !exact {
		for i, a := range c.Actions {
			clone := *a
			clone.Parameters = nil
			c.Actions[i] = &clone
		}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return bounded(string(b), e.MaxBytes)
}
