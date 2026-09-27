package turbo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolDefinitions deliberately keep action schemas out of the initial context.
func ToolDefinitions() []*mcp.Tool {
	definitions := []*mcp.Tool{
		{Name: "aws_discover", Description: "Find actions, presets, macros; exact action gives params.", InputSchema: json.RawMessage(`{"type":"object","properties":{"search":{"type":"string"}},"additionalProperties":false}`)},
		{Name: "aws_query", Description: "Read AWS; project and compress one page.", InputSchema: json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"},"params":{"type":"object"},"projection":{"type":"string"},"format":{"type":"string"}},"required":["action"],"additionalProperties":false}`)},
		{Name: "aws_mutate", Description: "Preview write; execute requires configured approval.", InputSchema: json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"},"params":{"type":"object"},"intent":{"type":"string"},"execute":{"type":"boolean"}},"required":["action","intent"],"additionalProperties":false}`)},
		{Name: "aws_diagnose", Description: "Run an evidence macro; discover names and params.", InputSchema: json.RawMessage(`{"type":"object","properties":{"macro":{"type":"string"},"params":{"type":"object"}},"required":["macro","params"],"additionalProperties":false}`)},
	}
	for _, tool := range definitions {
		if tool.Name != "aws_mutate" {
			tool.Annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
		}
	}
	return definitions
}

func NewServer(e *Engine, version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "aws-mcp-turbo", Version: version}, &mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{}})
	// Stdio and in-memory transports report an empty SDK Session.ID. Assign our
	// own identity per connection and release the mapping when it disconnects.
	var sessions sync.Map
	sessionID := func(session *mcp.ServerSession) string {
		if id, ok := sessions.Load(session); ok {
			return id.(string)
		}
		id, loaded := sessions.LoadOrStore(session, rand.Text())
		if !loaded {
			go func() { _ = session.Wait(); sessions.Delete(session) }()
		}
		return id.(string)
	}
	for _, tool := range ToolDefinitions() {
		s.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			text, err := dispatch(ctx, e, sessionID(req.Session), tool.Name, req.Params.Arguments)
			if err != nil {
				text = err.Error()
			}
			if len(text) > e.MaxBytes {
				text = "Response exceeds output limit; narrow the request."
				err = fmt.Errorf("output limit exceeded")
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: err != nil}, nil
		})
	}
	return s
}

func dispatch(ctx context.Context, e *Engine, session, name string, args json.RawMessage) (string, error) {
	if len(args) > 65536 {
		return "", fmt.Errorf("arguments exceed 64 KiB")
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if _, err := parameters(args); err != nil {
		return "", err
	}
	switch name {
	case "aws_discover":
		var in struct {
			Search string `json:"search"`
		}
		if err := decode(args, &in); err != nil {
			return "", err
		}
		return e.Discover(in.Search)
	case "aws_query":
		var in Query
		if err := decode(args, &in); err != nil {
			return "", err
		}
		return e.Query(ctx, session, in)
	case "aws_mutate":
		var in Mutation
		if err := decode(args, &in); err != nil {
			return "", err
		}
		return e.Mutate(ctx, session, in)
	case "aws_diagnose":
		var in struct {
			Macro  string          `json:"macro"`
			Params json.RawMessage `json:"params"`
		}
		if err := decode(args, &in); err != nil {
			return "", err
		}
		return e.Diagnose(ctx, session, in.Macro, in.Params)
	default:
		return "", fmt.Errorf("unknown tool")
	}
}
