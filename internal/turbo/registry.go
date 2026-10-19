package turbo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

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
	root       string                                              // JMESPath prefix that field selection applies to
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
