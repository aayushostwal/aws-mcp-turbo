package turbo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestMutationGates(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		enabled, execute, approve, auditFail bool
		wantCalls                            int
		wantError                            bool
	}{
		{"disabled", false, true, true, false, 0, true},
		{"preview", true, false, false, false, 0, false},
		{"no hook", true, true, false, false, 0, true},
		{"audit failure", true, true, true, true, 0, true},
		{"approved", true, true, true, false, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			a := fakeAction("ec2.StopInstances", "@", func(context.Context, json.RawMessage) (any, error) { calls++; return map[string]any{"ok": true}, nil })
			a.Write = true
			e := NewEngine(Registry{a.Name: a})
			var audit bytes.Buffer
			g := &MutationGuard{Enabled: tc.enabled, Audit: &audit}
			e.Mutations = g
			if tc.approve {
				g.Approve = func(_ context.Context, p []byte) error {
					if !bytes.Contains(p, []byte(`"InstanceIds":["i-1"]`)) || !bytes.Contains(p, []byte(`"intent":"maintenance"`)) {
						t.Fatalf("missing exact request: %s", p)
					}
					return nil
				}
			}
			if tc.auditFail {
				g.Audit = failingWriter{}
			}
			_, err := e.Mutate(context.Background(), "session", Mutation{Action: a.Name, Params: json.RawMessage(`{"InstanceIds":["i-1"]}`), Intent: "maintenance", Execute: tc.execute})
			if (err != nil) != tc.wantError || calls != tc.wantCalls {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if calls > 0 && !strings.Contains(audit.String(), `"status":"completed"`) {
				t.Fatal("missing completion audit")
			}
		})
	}
}

func TestMutationDeniedAndIntent(t *testing.T) {
	a := fakeAction("test.Write", "@", func(context.Context, json.RawMessage) (any, error) { t.Fatal("must not execute"); return nil, nil })
	a.Write = true
	e := NewEngine(Registry{a.Name: a})
	var audit bytes.Buffer
	e.Mutations = &MutationGuard{Enabled: true, Audit: &audit, Approve: func(context.Context, []byte) error { return errors.New("denied") }}
	if _, err := e.Mutate(context.Background(), "s", Mutation{Action: a.Name, Execute: true}); err == nil {
		t.Fatal("missing intent accepted")
	}
	if _, err := e.Mutate(context.Background(), "s", Mutation{Action: a.Name, Intent: "test", Execute: true}); err == nil || !strings.Contains(audit.String(), `"status":"denied"`) {
		t.Fatalf("%v %s", err, audit.String())
	}
}

func TestAuditFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	f, err := OpenAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	f, err = OpenAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAudit(path); err == nil {
		t.Fatal("public audit accepted")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAudit(link); err == nil {
		t.Fatal("symlink audit accepted")
	}
}

func TestApprovalHookContract(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`printf '{"approved":true}'`, true}, {`printf '{"approved":false}'`, false}, {`printf 'yes'`, false}, {`printf '{"approved":true}'; exit 1`, false},
	} {
		path := filepath.Join(t.TempDir(), "approve")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+tc.body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		hook, err := Hook(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := hook(context.Background(), []byte(`{}`)); (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.body, err)
		}
	}
	if _, err := Hook("relative-path"); err == nil {
		t.Fatal("relative hook accepted")
	}
}

func TestHookOutputLimitCannotBeBypassedByIOCopy(t *testing.T) {
	var output limitedBuffer
	reader := io.LimitReader(strings.NewReader(strings.Repeat("x", 2048)), 2048)
	if _, err := io.Copy(&output, reader); err == nil {
		t.Fatal("io.Copy bypassed the hook output limit")
	}
}
