package turbo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Mutation struct {
	Action  string          `json:"action"`
	Params  json.RawMessage `json:"params,omitempty"`
	Intent  string          `json:"intent"`
	Execute bool            `json:"execute,omitempty"`
}

type MutationGuard struct {
	Enabled bool
	Approve func(context.Context, []byte) error
	Audit   io.Writer
	mu      sync.Mutex
}

func OpenAudit(path string) (*os.File, error) {
	// O_EXCL creates private files; existing files must be regular and private.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		return f, nil
	}
	if !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("audit file must be a regular file with mode 0600")
	}
	f, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	actual, err := f.Stat()
	if err != nil || !os.SameFile(info, actual) {
		f.Close()
		return nil, fmt.Errorf("audit file changed while opening")
	}
	return f, nil
}

// Hook invokes an administrator-configured executable directly, without a shell.
// stdin is the exact audited request; only {"approved":true} with exit 0 permits execution.
func Hook(path string) (func(context.Context, []byte) error, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("approval hook must be an absolute executable path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil, fmt.Errorf("approval hook must be executable")
	}
	return func(ctx context.Context, payload []byte) error {
		cmd := exec.CommandContext(ctx, path)
		cmd.WaitDelay = time.Second
		cmd.Stdin = bytes.NewReader(payload)
		var output limitedBuffer
		cmd.Stdout = &output
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("approval hook failed: %w", err)
		}
		var answer struct {
			Approved bool `json:"approved"`
		}
		if err := decode(output.Bytes(), &answer); err != nil || !answer.Approved {
			return fmt.Errorf("approval denied")
		}
		return nil
	}, nil
}

// Do not embed bytes.Buffer: its promoted ReadFrom would let io.Copy bypass Write's limit.
type limitedBuffer struct{ buffer bytes.Buffer }

func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buffer.Len()+len(p) > 1024 {
		return 0, fmt.Errorf("hook output exceeds 1024 bytes")
	}
	return b.buffer.Write(p)
}

func (g *MutationGuard) log(payload []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Audit == nil {
		return fmt.Errorf("mutation audit is not configured")
	}
	line := append(append([]byte{}, payload...), '\n')
	n, err := g.Audit.Write(line)
	if err != nil {
		return fmt.Errorf("audit write: %w", err)
	}
	if n != len(line) {
		return io.ErrShortWrite
	}
	if syncer, ok := g.Audit.(interface{ Sync() error }); ok {
		return syncer.Sync()
	}
	return nil
}

func (e *Engine) Mutate(ctx context.Context, session string, m Mutation) (string, error) {
	g := e.Mutations
	if g == nil || !g.Enabled {
		return "", fmt.Errorf("mutations are disabled; start with --enable-mutations and --audit-file")
	}
	if strings.TrimSpace(m.Intent) == "" {
		return "", fmt.Errorf("intent is required")
	}
	a, err := e.Registry.Find(m.Action)
	if err != nil {
		return "", err
	}
	if !a.Write {
		return "", fmt.Errorf("read-only action requires aws_query")
	}
	p, err := parameters(m.Params)
	if err != nil {
		return "", err
	}
	m.Params, _ = json.Marshal(p)
	if a.Validate != nil {
		if err := a.Validate(m.Params); err != nil {
			return "", err
		}
	}
	record := map[string]any{"time": time.Now().UTC(), "session": session, "request": m, "status": "requested"}
	payload, _ := json.Marshal(record)
	if err := g.log(payload); err != nil {
		return "", err
	}
	if !m.Execute {
		return "Local preview only; no AWS call made. request=" + string(mustJSON(m)), nil
	}
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	if g.Approve == nil {
		return "", fmt.Errorf("execution requires --approval-hook")
	}
	if err := g.Approve(ctx, payload); err != nil {
		record["status"] = "denied"
		b, _ := json.Marshal(record)
		if logErr := g.log(b); logErr != nil {
			return "", logErr
		}
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	record["status"] = "approved"
	approved, _ := json.Marshal(record)
	if err := g.log(approved); err != nil {
		return "", err
	}
	raw, callErr := a.Call(ctx, m.Params)
	record["status"] = "completed"
	if callErr != nil {
		record["status"] = "aws_error"
		record["error"] = callErr.Error()
	}
	b, _ := json.Marshal(record)
	if err := g.log(b); err != nil {
		return "", fmt.Errorf("AWS call attempted, outcome audit failed; verify AWS state before retrying: %w", err)
	}
	if callErr != nil {
		return "", callErr
	}
	projected, err := a.expression.Search(raw)
	if err != nil {
		return "", fmt.Errorf("AWS call succeeded but projection failed; verify state before retrying: %w", err)
	}
	text, err := Format(projected, "", e.MaxBytes)
	if err != nil {
		return "", fmt.Errorf("AWS call succeeded but output failed; verify state before retrying: %w", err)
	}
	return text, nil
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
