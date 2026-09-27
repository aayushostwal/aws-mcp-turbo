package turbo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/jmespath/go-jmespath"
)

// Action is an explicit allowlist entry. An API's name never determines its safety.
type Action struct {
	Name       string                                              `json:"action"`
	Projection string                                              `json:"projection"`
	Required   []string                                            `json:"required,omitempty"`
	Parameters map[string]string                                   `json:"parameters,omitempty"`
	Write      bool                                                `json:"write,omitempty"`
	TokenOut   string                                              `json:"-"`
	TokenIn    string                                              `json:"-"`
	Validate   func(json.RawMessage) error                         `json:"-"`
	Call       func(context.Context, json.RawMessage) (any, error) `json:"-"`
	expression *jmespath.JMESPath
}

type Registry map[string]*Action

func (r Registry) Find(name string) (*Action, error) {
	a, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("unsupported action %q; use aws_discover", name)
	}
	return a, nil
}

func (r Registry) Discover(search string) []*Action {
	var found []*Action
	for _, a := range r {
		if strings.Contains(strings.ToLower(a.Name), strings.ToLower(search)) {
			found = append(found, a)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	return found
}

func decode(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}

func parameters(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil || p == nil {
		return nil, fmt.Errorf("params must be an object")
	}
	return p, nil
}

// bind preserves SDK validation and wire encoding while accepting AWS Go field names.
func bind[I, O, Opt any](r Registry, a Action, fn func(context.Context, *I, ...func(*Opt)) (*O, error)) {
	a.expression = jmespath.MustCompile(a.Projection)
	a.Parameters = map[string]string{}
	t := reflect.TypeFor[I]()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.IsExported() {
			a.Parameters[f.Name] = f.Type.String()
		}
	}
	parse := func(raw json.RawMessage) (*I, error) {
		p, err := parameters(raw)
		if err != nil {
			return nil, err
		}
		// encoding/json matches fields case-insensitively. Reject aliases so cache
		// keys, approval payloads and the SDK always refer to the same fields.
		for key := range p {
			if _, ok := a.Parameters[key]; !ok {
				return nil, fmt.Errorf("unknown parameter %q (field names are case-sensitive)", key)
			}
		}
		for _, key := range a.Required {
			v, ok := p[key]
			if !ok || v == nil || v == "" {
				return nil, fmt.Errorf("missing required parameter %s", key)
			}
			if arr, ok := v.([]any); ok && len(arr) == 0 {
				return nil, fmt.Errorf("%s must not be empty", key)
			}
		}
		b, _ := json.Marshal(p)
		var in I
		if err := decode(b, &in); err != nil {
			return nil, fmt.Errorf("invalid AWS parameters: %w", err)
		}
		return &in, nil
	}
	a.Validate = func(raw json.RawMessage) error { _, err := parse(raw); return err }
	a.Call = func(ctx context.Context, raw json.RawMessage) (any, error) {
		in, err := parse(raw)
		if err != nil {
			return nil, err
		}
		out, err := fn(ctx, in)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		var normalized any
		err = json.Unmarshal(b, &normalized)
		return normalized, err
	}
	r[a.Name] = &a
}
