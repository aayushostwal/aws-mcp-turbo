package turbo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jmespath/go-jmespath"
)

type serviceDef struct {
	name string
	typ  reflect.Type
	make func(aws.Config) any

	once sync.Once
	ops  map[string]*operation
	list []*operation // sorted by name
}

func def[C any](name string, make func(aws.Config) *C) *serviceDef {
	return &serviceDef{name: name, typ: reflect.TypeFor[*C](), make: func(c aws.Config) any { return make(c) }}
}

type operation struct {
	name   string // service.Operation
	op     string
	method reflect.Method
	in     reflect.Type // struct
	out    reflect.Type // struct
	write  bool
}

// ServiceSet resolves "service.Operation" names against the AWS SDK on demand.
// Clients are created per service and region on first use and then reused.
type ServiceSet struct {
	cfg      aws.Config
	services map[string]*serviceDef
	names    []string

	mu      sync.Mutex
	clients map[string]any
	actions sync.Map // name -> *Action
}

func NewServiceSet(cfg aws.Config) *ServiceSet {
	s := &ServiceSet{cfg: cfg, services: map[string]*serviceDef{}, clients: map[string]any{}}
	for _, d := range serviceDefs() {
		s.services[d.name] = d
		s.names = append(s.names, d.name)
	}
	sort.Strings(s.names)
	return s
}

type regionKey struct{}

func withRegion(ctx context.Context, region string) context.Context {
	if region == "" {
		return ctx
	}
	return context.WithValue(ctx, regionKey{}, region)
}

func (s *ServiceSet) client(ctx context.Context, d *serviceDef) any {
	region, _ := ctx.Value(regionKey{}).(string)
	if region == "" {
		region = s.cfg.Region
	}
	key := d.name + "\x00" + region
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.clients[key]; ok {
		return c
	}
	cfg := s.cfg
	cfg.Region = region
	c := d.make(cfg)
	s.clients[key] = c
	return c
}

var (
	ctxType   = reflect.TypeFor[context.Context]()
	errType   = reflect.TypeFor[error]()
	readerTyp = reflect.TypeFor[io.Reader]()
)

// operations indexes the SDK client's API methods. Only the type is inspected;
// no client is built.
func (d *serviceDef) operations() map[string]*operation {
	d.once.Do(func() {
		d.ops = map[string]*operation{}
		for i := 0; i < d.typ.NumMethod(); i++ {
			m := d.typ.Method(i)
			t := m.Type // func(recv, ctx, *In, ...func(*Options)) (*Out, error)
			if t.NumIn() != 4 || !t.IsVariadic() || t.NumOut() != 2 || t.In(1) != ctxType || t.Out(1) != errType {
				continue
			}
			in, out := t.In(2), t.Out(0)
			if in.Kind() != reflect.Pointer || in.Elem().Kind() != reflect.Struct || out.Kind() != reflect.Pointer || out.Elem().Kind() != reflect.Struct {
				continue
			}
			name := d.name + "." + m.Name
			if denied[name] || streams(in.Elem()) || streams(out.Elem()) {
				continue
			}
			d.ops[m.Name] = &operation{name: name, op: m.Name, method: m, in: in.Elem(), out: out.Elem(), write: !isRead(d.name, m.Name)}
		}
		for _, o := range d.ops {
			d.list = append(d.list, o)
		}
		sort.Slice(d.list, func(i, j int) bool { return d.list[i].op < d.list[j].op })
	})
	return d.ops
}

// streams reports structs that carry raw byte streams, which JSON cannot express.
func streams(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.IsExported() && (f.Type.Implements(readerTyp) || strings.HasSuffix(f.Type.String(), "EventStream")) {
			return true
		}
	}
	return false
}

var readPrefixes = []string{"List", "Describe", "Get", "Lookup", "Search", "BatchGet", "Query", "Scan", "Simulate", "Validate", "Estimate", "Head", "Check", "Filter", "Decode", "Preview", "AdminGet", "AdminList"}

// readExceptions are side-effect free despite their verb.
var readExceptions = map[string]bool{"logs.StartQuery": true}

// writeExceptions look like reads but change state or reveal credentials.
var writeExceptions = map[string]bool{"sts.GetSessionToken": true}

func isRead(service, op string) bool {
	name := service + "." + op
	if v, ok := readExceptions[name]; ok {
		return v
	}
	if writeExceptions[name] {
		return false
	}
	for _, p := range readPrefixes {
		if strings.HasPrefix(op, p) {
			return true
		}
	}
	return false
}

// denied operations return credentials, key material or raw secrets. They are
// unavailable even through the approval-gated mutation path.
var denied = map[string]bool{
	"secretsmanager.GetSecretValue": true, "secretsmanager.BatchGetSecretValue": true,
	"ec2.GetPasswordData": true, "sts.GetSessionToken": true, "sts.GetFederationToken": true,
	"sts.AssumeRole": true, "sts.AssumeRoleWithSAML": true, "sts.AssumeRoleWithWebIdentity": true, "sts.AssumeRoot": true,
	"ecr.GetAuthorizationToken": true, "ecr.GetDownloadUrlForLayer": true,
	"redshift.GetClusterCredentials": true, "redshift.GetClusterCredentialsWithIAM": true,
	"kms.Decrypt": true, "kms.GenerateDataKey": true, "kms.GenerateDataKeyWithoutPlaintext": true,
	"kms.GenerateDataKeyPair": true, "kms.GenerateDataKeyPairWithoutPlaintext": true,
	"iam.CreateAccessKey": true, "iam.CreateServiceSpecificCredential": true, "iam.ResetServiceSpecificCredential": true,
	"cognito.GetCredentialsForIdentity": true, "cognito.GetOpenIdToken": true,
}

// guardedFlags are parameters that turn a harmless read into a secret read.
var guardedFlags = map[string]any{"WithDecryption": true, "IncludeValue": true, "IncludeValues": true, "IncludePlaintext": true, "HidePassword": false}

func guard(p map[string]any) error {
	for k, bad := range guardedFlags {
		if v, ok := p[k]; ok && v == bad {
			return fmt.Errorf("parameter %s=%v is not allowed; it can reveal secret values", k, bad)
		}
	}
	return nil
}

// Resolve returns the action for "service.Operation", building it on first use.
func (s *ServiceSet) Resolve(name string) (*Action, error) {
	if a, ok := s.actions.Load(name); ok {
		return a.(*Action), nil
	}
	svc, op, ok := strings.Cut(name, ".")
	d := s.services[svc]
	if !ok || d == nil {
		return nil, fmt.Errorf("unsupported action %q; use aws_discover", name)
	}
	o := d.operations()[op]
	if o == nil {
		return nil, fmt.Errorf("unsupported action %q; use aws_discover with a search term", name)
	}
	a := s.build(d, o)
	actual, _ := s.actions.LoadOrStore(name, a)
	return actual.(*Action), nil
}

func (s *ServiceSet) build(d *serviceDef, o *operation) *Action {
	a := &Action{Name: o.name, Write: o.write, Parameters: map[string]string{}}
	for i := 0; i < o.in.NumField(); i++ {
		if f := o.in.Field(i); f.IsExported() {
			a.Parameters[f.Name] = typeLabel(f.Type)
		}
	}
	a.TokenOut, a.TokenIn = paginationFields(o.in, o.out)
	a.root = rootPath(o.out)
	if p, ok := presets[o.name]; ok {
		a.Projection, a.Required = p.Projection, p.Required
		if p.Root != "" {
			a.root = p.Root
		}
		if p.TokenOut != "" {
			a.TokenOut, a.TokenIn = p.TokenOut, p.TokenIn
		}
	}
	if a.Projection == "" {
		a.Projection = autoProjection(o.out)
	}
	if expr, err := jmespath.Compile(a.Projection); err == nil {
		a.expression = expr
	} else {
		a.Projection = "@"
		a.expression = jmespath.MustCompile("@")
	}
	parse := func(raw json.RawMessage) (reflect.Value, error) {
		p, err := parameters(raw)
		if err != nil {
			return reflect.Value{}, err
		}
		// encoding/json matches fields case-insensitively. Reject aliases so cache
		// keys, approval payloads and the SDK always refer to the same fields.
		for key := range p {
			if _, ok := a.Parameters[key]; !ok {
				return reflect.Value{}, fmt.Errorf("unknown parameter %q (field names are case-sensitive); see aws_discover action=%s", key, a.Name)
			}
		}
		if err := guard(p); err != nil {
			return reflect.Value{}, err
		}
		for _, key := range a.Required {
			v, ok := p[key]
			if !ok || v == nil || v == "" {
				return reflect.Value{}, fmt.Errorf("missing required parameter %s", key)
			}
			if arr, ok := v.([]any); ok && len(arr) == 0 {
				return reflect.Value{}, fmt.Errorf("%s must not be empty", key)
			}
		}
		b, _ := json.Marshal(p)
		in := reflect.New(o.in)
		if err := decode(b, in.Interface()); err != nil {
			// Inputs holding interface unions (DynamoDB items) need the typed decoder.
			in = reflect.New(o.in)
			if typedErr := decodeTyped(b, in.Interface()); typedErr != nil {
				return reflect.Value{}, fmt.Errorf("invalid AWS parameters: %w", typedErr)
			}
		}
		return in, nil
	}
	a.Validate = func(raw json.RawMessage) error { _, err := parse(raw); return err }
	a.Call = func(ctx context.Context, raw json.RawMessage) (any, error) {
		in, err := parse(raw)
		if err != nil {
			return nil, err
		}
		res := o.method.Func.Call([]reflect.Value{reflect.ValueOf(s.client(ctx, d)), reflect.ValueOf(ctx), in})
		if e, _ := res[1].Interface().(error); e != nil {
			return nil, e
		}
		b, err := json.Marshal(res[0].Interface())
		if err != nil {
			return nil, err
		}
		var normalized any
		if err := json.Unmarshal(b, &normalized); err != nil {
			return nil, err
		}
		normalized = unwrapUnions(normalized)
		// A DynamoDB continuation key must stay in request form to be passed back.
		if obj, ok := normalized.(map[string]any); ok && a.TokenOut != "" {
			if f := res[0].Elem().FieldByName(a.TokenOut); f.IsValid() && f.Kind() == reflect.Map {
				if key, ok := f.Interface().(map[string]dynamodbtypes.AttributeValue); ok {
					obj[a.TokenOut] = keyWire(key)
				}
			}
		}
		return normalized, nil
	}
	return a
}

// unwrapUnions turns SDK union members ({"Value": x}) into plain values, which
// makes DynamoDB items and similar tagged types readable.
func unwrapUnions(v any) any {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 1 {
			if inner, ok := x["Value"]; ok {
				return unwrapUnions(inner)
			}
		}
		for k, e := range x {
			x[k] = unwrapUnions(e)
		}
	case []any:
		for i, e := range x {
			x[i] = unwrapUnions(e)
		}
	}
	return v
}

func typeLabel(t reflect.Type) string {
	s := t.String()
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// paginationFields finds the response token and the request field it feeds.
func paginationFields(in, out reflect.Type) (tokenOut, tokenIn string) {
	has := func(t reflect.Type, name string) bool { _, ok := t.FieldByName(name); return ok }
	for i := 0; i < out.NumField(); i++ {
		f := out.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Name
		isToken := strings.HasPrefix(name, "Next") && (name == "NextToken" || strings.HasSuffix(name, "Token") || strings.HasSuffix(name, "Marker")) ||
			name == "Marker" || name == "Position" || name == "LastEvaluatedKey" || name == "LastEvaluatedTableName"
		if !isToken || (f.Type.Kind() != reflect.Pointer && f.Type.Kind() != reflect.Map && f.Type.Kind() != reflect.String) {
			continue
		}
		candidates := []string{name, strings.TrimPrefix(name, "Next"), strings.Replace(name, "LastEvaluated", "ExclusiveStart", 1)}
		for _, c := range candidates {
			if has(in, c) {
				return name, c
			}
		}
	}
	return "", ""
}

var (
	idLike    = regexp.MustCompile(`(^|[a-z0-9])(Id|ID|Name|Arn|ARN|Identifier)$`)
	stateLike = regexp.MustCompile(`(State|Status|Phase|Health|Type|Class|Engine|Version|Runtime|Mode|Enabled)`)
	metaLike  = regexp.MustCompile(`(Creat|Modif|Updat|LastUpdate|LastModif|Endpoint|Address|Uri|URI|Url|URL|DNS|SizeIn|Size$|Count$|Capacity$|Region|Zone$)`)
	noisy     = regexp.MustCompile(`(Token|Marker|Metadata|Signature|Checksum|ETag|Etag|Document|Policy|Description|Reason|Message)$`)
	stateKeys = []string{"Name", "Code", "Status", "State", "Value"}
)

const maxPicks = 8

type pick struct {
	alias, expr string
	rank        int // lower sorts first within its group
}

func isScalar(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	case reflect.Struct:
		return t.PkgPath() == "time" && t.Name() == "Time"
	}
	return false
}

func elemStruct(t reflect.Type) (reflect.Type, bool) {
	if t.Kind() != reflect.Slice {
		return nil, false
	}
	e := t.Elem()
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}
	return e, e.Kind() == reflect.Struct && !isScalar(e)
}

func isStringList(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.String
}

func structOf(t reflect.Type) (reflect.Type, bool) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t, t.Kind() == reflect.Struct && !isScalar(t)
}

// stem strips a trailing identifier word: SubnetId -> Subnet.
func stem(name string) string {
	for _, suffix := range []string{"Identifier", "Name", "Arn", "ARN", "Id", "ID"} {
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return ""
}

// stateField looks inside a small record such as InstanceState for its label.
func stateField(t reflect.Type) string {
	st, ok := structOf(t)
	if !ok {
		return ""
	}
	for _, key := range stateKeys {
		if f, ok := st.FieldByName(key); ok && isScalar(f.Type) {
			return key
		}
	}
	return ""
}

// pickFields chooses the few fields of a record worth showing by default:
// its own identifiers first, then state, then a couple of timestamps or endpoints.
func pickFields(t reflect.Type) []pick {
	var own, other, states, meta, rest []pick
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || noisy.MatchString(f.Name) {
			continue
		}
		switch {
		case isScalar(f.Type) || isStringList(f.Type):
			p := pick{alias: f.Name, expr: f.Name, rank: 3}
			if tm, _ := structOf(f.Type); tm.PkgPath() == "time" {
				p.rank = 0 // timestamps beat other metadata
			}
			switch s := stem(f.Name); {
			case f.Name == "Id" || f.Name == "ID" || f.Name == "Name" || f.Name == "Arn" || f.Name == "ARN" || f.Name == "Value" || f.Name == "Key":
				own = append(own, p)
			case s != "" && (strings.HasPrefix(t.Name(), s) || strings.HasPrefix(s, t.Name())):
				own = append(own, p)
			case idLike.MatchString(f.Name):
				other = append(other, p)
			case stateLike.MatchString(f.Name):
				switch {
				case f.Name == "State" || f.Name == "Status":
					p.rank = 0
				case strings.HasSuffix(f.Name, "State") || strings.HasSuffix(f.Name, "Status"):
					p.rank = 1
				case strings.HasSuffix(f.Name, "Type") || strings.HasSuffix(f.Name, "Version") || strings.HasSuffix(f.Name, "Class") || strings.HasSuffix(f.Name, "Engine"):
					p.rank = 2
				}
				states = append(states, p)
			case metaLike.MatchString(f.Name):
				meta = append(meta, p)
			default:
				rest = append(rest, p)
			}
		case stateLike.MatchString(f.Name):
			if key := stateField(f.Type); key != "" {
				states = append(states, pick{alias: f.Name, expr: f.Name + "." + key, rank: 1})
			}
		}
	}
	// Identifiers: Name and Id before Arn, and the Arn only if nothing else names the record.
	sort.SliceStable(own, func(i, j int) bool { return rankOwn(own[i].alias) < rankOwn(own[j].alias) })
	if len(own) == 0 {
		own = other
	}
	if len(own) > 3 {
		own = own[:3]
	}
	byRank := func(p []pick) []pick {
		sort.SliceStable(p, func(i, j int) bool { return p[i].rank < p[j].rank })
		return p
	}
	states, meta = byRank(states), byRank(meta)
	capTo := func(p []pick, n int) []pick {
		if len(p) > n {
			return p[:n]
		}
		return p
	}
	picks := append(append(append([]pick{}, own...), capTo(states, 3)...), capTo(meta, 2)...)
	if len(picks) < 3 {
		picks = append(picks, capTo(rest, 3-len(picks))...)
	}
	if len(picks) > maxPicks {
		picks = picks[:maxPicks]
	}
	return picks
}

func rankOwn(name string) int {
	switch {
	case strings.HasSuffix(name, "Arn") || strings.HasSuffix(name, "ARN"):
		return 2
	case strings.HasSuffix(name, "Name"):
		return 1
	}
	return 0
}

func multiselect(picks []pick) string {
	parts := make([]string, len(picks))
	for i, p := range picks {
		parts[i] = p.alias + ":" + p.expr
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// listPath returns a JMESPath prefix for a list of records. A parent with nothing
// worth showing of its own is looked through to the one list inside it.
func listPath(name string, elem reflect.Type) (string, reflect.Type) {
	path := name + "[]"
	for depth := 0; depth < 2 && len(pickFields(elem)) == 0; depth++ {
		var nested []string
		var nestedElem reflect.Type
		for i := 0; i < elem.NumField(); i++ {
			f := elem.Field(i)
			if e, ok := elemStruct(f.Type); ok && f.IsExported() {
				nested = append(nested, f.Name)
				nestedElem = e
			}
		}
		if len(nested) != 1 {
			break
		}
		path += "." + nested[0] + "[]"
		elem = nestedElem
	}
	return path, elem
}

// wrapperList recognises records like CloudFront's DistributionList, which only
// carry paging fields around one list of items.
func wrapperList(t reflect.Type) (field string, elem reflect.Type, ok bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if e, isList := elemStruct(f.Type); isList {
			if field != "" {
				return "", nil, false
			}
			field, elem = f.Name, e
		} else if !skipTop.MatchString(f.Name) && f.Name != "Items" {
			return "", nil, false
		}
	}
	return field, elem, field != ""
}

var skipTop = regexp.MustCompile(`^(ResultMetadata|Failures|Errors|Unprocessed.*|.*Token|.*Marker|NextPosition|Position|LastEvaluated.*|IsTruncated|HasMore.*|MaxItems|MaxResults|Quantity|.*Error|.*Errors)$`)

// autoProjection derives a compact default view from an output type: identifiers,
// names, state and a few key attributes.
func autoProjection(out reflect.Type) string {
	type entry struct {
		name, expr string
		payload    bool // a list or record, as opposed to a bare scalar
	}
	var entries []entry
	for i := 0; i < out.NumField(); i++ {
		f := out.Field(i)
		if !f.IsExported() || skipTop.MatchString(f.Name) {
			continue
		}
		switch {
		case isRecordList(f.Type):
			// Free-form records (DynamoDB Items) are the whole answer.
			return f.Name
		case isScalar(f.Type):
			entries = append(entries, entry{f.Name, f.Name, false})
		case isStringList(f.Type):
			entries = append(entries, entry{f.Name, f.Name, true})
		default:
			if e, ok := elemStruct(f.Type); ok {
				path, leaf := listPath(f.Name, e)
				if picks := pickFields(leaf); len(picks) > 0 {
					entries = append(entries, entry{f.Name, path + "." + multiselect(picks), true})
				}
			} else if st, ok := structOf(f.Type); ok {
				if field, e, wrapped := wrapperList(st); wrapped {
					path, leaf := listPath(f.Name+"."+field, e)
					if picks := pickFields(leaf); len(picks) > 0 {
						entries = append(entries, entry{f.Name, path + "." + multiselect(picks), true})
					}
				} else if picks := pickFields(st); len(picks) > 0 {
					entries = append(entries, entry{f.Name, f.Name + "." + multiselect(picks), true})
				}
			}
		}
	}
	// Scalars next to a list or record are paging and bookkeeping; drop them.
	hasPayload := false
	for _, e := range entries {
		hasPayload = hasPayload || e.payload
	}
	if hasPayload {
		kept := entries[:0]
		for _, e := range entries {
			if e.payload {
				kept = append(kept, e)
			}
		}
		entries = kept
	}
	if len(entries) > maxPicks {
		entries = entries[:maxPicks]
	}
	switch len(entries) {
	case 0:
		return "@"
	case 1:
		return entries[0].expr
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = e.name + ":" + e.expr
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// Search lists operations whose "service.Operation" name contains every term.
// A bare service name lists that service's reads first.
func (s *ServiceSet) Search(terms []string, limit int) (names []string, writes map[string]bool, total int) {
	writes = map[string]bool{}
	match := func(n string) bool {
		l := strings.ToLower(n)
		for _, t := range terms {
			if !strings.Contains(l, t) {
				return false
			}
		}
		return true
	}
	var reads, wr []string
	for _, svc := range s.names {
		d := s.services[svc]
		if len(terms) > 0 && !s.serviceMayMatch(svc, terms) {
			continue
		}
		d.operations()
		for _, o := range d.list {
			if !match(o.name) {
				continue
			}
			if o.write {
				wr = append(wr, o.name)
			} else {
				reads = append(reads, o.name)
			}
		}
	}
	all := append(reads, wr...)
	total = len(all)
	for _, n := range wr {
		writes[n] = true
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, writes, total
}

// serviceMayMatch avoids indexing services that cannot contain a match. A term
// equal to a service name, or "service.prefix", restricts the search to it.
func (s *ServiceSet) serviceMayMatch(svc string, terms []string) bool {
	restricted := false
	for _, t := range terms {
		name, _, dotted := strings.Cut(t, ".")
		if _, known := s.services[name]; known && (dotted || t == name) {
			restricted = true
			if name == svc {
				return true
			}
		}
	}
	return !restricted
}

// Describe returns accepted parameters and returned fields for one action.
func (s *ServiceSet) Describe(a *Action) (returns []string) {
	svc, op, _ := strings.Cut(a.Name, ".")
	if o := s.services[svc].operations()[op]; o != nil {
		for i := 0; i < o.out.NumField(); i++ {
			if f := o.out.Field(i); f.IsExported() && f.Name != "ResultMetadata" {
				returns = append(returns, f.Name)
			}
		}
	}
	return returns
}

// rootPath is where a response keeps its main content: the one list of records
// or the one record. It is empty when the response has no single main part.
func rootPath(out reflect.Type) string {
	var roots []string
	for i := 0; i < out.NumField(); i++ {
		f := out.Field(i)
		if !f.IsExported() || skipTop.MatchString(f.Name) {
			continue
		}
		if isRecordList(f.Type) {
			return f.Name + "[]"
		}
		if e, ok := elemStruct(f.Type); ok {
			path, _ := listPath(f.Name, e)
			roots = append(roots, path)
		} else if st, ok := structOf(f.Type); ok && !isScalar(f.Type) {
			if field, e, wrapped := wrapperList(st); wrapped {
				path, _ := listPath(f.Name+"."+field, e)
				roots = append(roots, path)
			} else {
				roots = append(roots, f.Name)
			}
		}
	}
	if len(roots) == 1 {
		return roots[0]
	}
	return ""
}

var fieldPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// fieldsProjection builds a projection that keeps only the named fields, and
// returns their names in the order asked. A dotted path such as State.Name
// reaches into a nested record and keeps that path as its column name.
func fieldsProjection(root string, fields []string) (string, []string, error) {
	seen := map[string]bool{}
	var parts, order []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		if !fieldPath.MatchString(f) {
			return "", nil, fmt.Errorf("invalid field %q; use names like Id or State.Name", f)
		}
		seen[f] = true
		order = append(order, f)
		if strings.Contains(f, ".") {
			parts = append(parts, `"`+f+`":`+f)
		} else {
			parts = append(parts, f+":"+f)
		}
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("fields is empty")
	}
	sel := "{" + strings.Join(parts, ",") + "}"
	if root == "" {
		return sel, order, nil
	}
	return root + "." + sel, order, nil
}

// isRecordList matches lists of free-form records such as DynamoDB items.
func isRecordList(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Map
}

func compileProjection(p string) (*jmespath.JMESPath, error) { return jmespath.Compile(p) }
