// measure compares actual tool output with synthetic, SDK-shaped fixtures.
// It never contacts AWS and is not shipped in release binaries.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aayushostwal/aws-mcp-turbo/internal/turbo"
	"github.com/aws/aws-sdk-go-v2/aws"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	b, err := os.ReadFile("testdata/payloads.json")
	if err != nil {
		return err
	}
	var fixtures []struct {
		Action   string `json:"action"`
		Response any    `json:"response"`
	}
	if err := json.Unmarshal(b, &fixtures); err != nil {
		return err
	}
	e := turbo.NewEngine(turbo.AWSRegistry(aws.Config{}))
	var results []map[string]any
	for _, f := range fixtures {
		a := e.Registry[f.Action]
		if a == nil {
			return fmt.Errorf("unknown fixture action %s", f.Action)
		}
		a.Call = func(context.Context, json.RawMessage) (any, error) { return f.Response, nil }
		a.Validate = nil
		out, err := e.Query(context.Background(), "benchmark", turbo.Query{Action: f.Action})
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(f.Response)
		results = append(results, map[string]any{"action": f.Action, "raw": string(raw), "compressed": out})
	}
	return json.NewEncoder(os.Stdout).Encode(results)
}
