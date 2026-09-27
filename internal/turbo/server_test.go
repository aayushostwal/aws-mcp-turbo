package turbo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPIntegration(t *testing.T) {
	ctx := context.Background()
	a := fakeAction("test.Read", "Items", func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"Items": []any{map[string]any{"Id": "one"}}}, nil
	})
	e := NewEngine(Registry{a.Name: a})
	server := NewServer(e, "test")
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 4 {
		t.Fatalf("tool count %d", len(list.Tools))
	}
	for _, tool := range list.Tools {
		if tool.Name == "aws_query" {
			encoded, _ := json.Marshal(tool.InputSchema)
			if !strings.Contains(string(encoded), `"region"`) {
				t.Fatal("aws_query does not advertise the region argument")
			}
		}
	}
	for _, tc := range []struct {
		name      string
		args      map[string]any
		wantError bool
	}{
		{"aws_discover", map[string]any{}, false},
		{"aws_query", map[string]any{"action": "test.Read"}, false},
		{"aws_query", map[string]any{"action": "test.Read", "unknown": true}, true},
		{"aws_query", map[string]any{"action": "test.Read", "projection": "["}, true},
		{"aws_mutate", map[string]any{"action": "test.Write", "intent": "test", "execute": true}, true},
		{"aws_diagnose", map[string]any{"macro": "missing", "params": map[string]any{}}, true},
	} {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != tc.wantError {
			t.Fatalf("%s: %+v", tc.name, result)
		}
		if len(result.Content) != 1 || result.StructuredContent != nil {
			t.Fatal("duplicated/raw structured payload")
		}
	}
}

func TestDispatchLimits(t *testing.T) {
	e := NewEngine(Registry{})
	for _, s := range []string{`null`, `[]`, `{} {}`, strings.Repeat(" ", 65537)} {
		if _, err := dispatch(context.Background(), e, "s", "aws_discover", json.RawMessage(s)); err == nil {
			t.Fatalf("accepted %q", s[:min(len(s), 40)])
		}
	}
}

func TestMCPConnectionsHaveIsolatedDeltaState(t *testing.T) {
	ctx := context.Background()
	a := fakeAction("logs.GetLogEvents", "Events", func(_ context.Context, p json.RawMessage) (any, error) {
		args, _ := parameters(p)
		message := "first"
		if args["NextToken"] != nil {
			message = "second"
		}
		return map[string]any{"Events": []any{message}, "NextForwardToken": "cursor"}, nil
	})
	a.TokenOut = "NextForwardToken"
	s := NewServer(NewEngine(Registry{a.Name: a}), "test")
	connect := func() *mcp.ClientSession {
		st, ct := mcp.NewInMemoryTransports()
		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = ss.Close() })
		c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		cs, err := c.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cs.Close() })
		return cs
	}
	one, two := connect(), connect()
	call := func(c *mcp.ClientSession) string {
		out, err := c.CallTool(ctx, &mcp.CallToolParams{Name: "aws_query", Arguments: map[string]any{"action": a.Name}})
		if err != nil || out.IsError {
			t.Fatalf("%+v %v", out, err)
		}
		return out.Content[0].(*mcp.TextContent).Text
	}
	if !strings.Contains(call(one), "first") || !strings.Contains(call(one), "second") || !strings.Contains(call(two), "first") {
		t.Fatal("MCP sessions share polling state")
	}
}
