package turbo

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jmespath/go-jmespath"
)

func TestPrune(t *testing.T) {
	input := map[string]any{"nil": nil, "arr": []any{}, "obj": map[string]any{}, "recursive": map[string]any{"a": []any{nil}}, "false": false, "zero": 0, "empty": "", "ResultMetadata": map[string]any{"RequestID": "x"}}
	want := map[string]any{"false": false, "zero": 0, "empty": ""}
	if got := Prune(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
	if len(input) != 8 {
		t.Fatal("prune mutated input")
	}
}

func TestFormats(t *testing.T) {
	rows := []any{map[string]any{"b": false, "a": "x|y\tz\nnext\\"}, map[string]any{"a": "other", "c": 0}}
	md, err := Format(rows, "", 32768)
	if err != nil || !strings.HasPrefix(md, "|a|b|c|\n|---|---|---|") || !strings.Contains(md, `x\|y\tz\nnext\\`) {
		t.Fatalf("%q %v", md, err)
	}
	tsv, err := Format(rows, "tsv", 32768)
	if err != nil || len(strings.Split(tsv, "\n")) != 3 || !strings.HasPrefix(tsv, "a\tb\tc\n") {
		t.Fatalf("%q %v", tsv, err)
	}
	js, err := Format(rows, "json", 32768)
	if err != nil || !json.Valid([]byte(js)) {
		t.Fatalf("%q %v", js, err)
	}
	if _, err := Format(rows, "markdown", 8); err == nil {
		t.Fatal("unbounded output")
	}
	if s, _ := Format([]any{}, "", 100); s != "No results." {
		t.Fatal(s)
	}
	if s, _ := Format([]any{"a", "b"}, "", 100); !strings.Contains(s, "|Value|") {
		t.Fatal(s)
	}
}

func BenchmarkProjectionFormatting(b *testing.B) {
	rows := make([]any, 1000)
	for i := range rows {
		rows[i] = map[string]any{"Id": "i-0123456789", "State": "running", "PrivateIp": "10.0.0.1", "Unused": nil}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Format(rows, "", 1024*1024); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueryPipeline(b *testing.B) {
	rows := make([]any, 1000)
	for i := range rows {
		rows[i] = map[string]any{"InstanceId": "i-0123456789", "State": map[string]any{"Name": "running"}, "PrivateIpAddress": "10.0.0.1", "NetworkInterfaces": []any{}, "Hypervisor": "xen"}
	}
	a := fakeAction("ec2.DescribeInstances", "Instances[].{Id:InstanceId,State:State.Name,IP:PrivateIpAddress}", func(context.Context, json.RawMessage) (any, error) { return map[string]any{"Instances": rows}, nil })
	a.expression = jmespath.MustCompile(a.Projection)
	e := NewEngine(Registry{a.Name: a})
	e.MaxBytes = 1024 * 1024
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.Query(context.Background(), "bench", Query{Action: a.Name}); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzFormat(f *testing.F) {
	f.Add(`[{"a":"hello|world\nnext","b":false},null,{}]`)
	f.Fuzz(func(t *testing.T, s string) {
		var v any
		if json.Unmarshal([]byte(s), &v) != nil {
			return
		}
		for _, format := range []string{"markdown", "tsv", "json"} {
			result, err := Format(v, format, 4096)
			if err == nil && len(result) > 4096 {
				t.Fatal("limit exceeded")
			}
		}
	})
}
