package turbo

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// The comparison tasks are documentation people copy, so every turbo call in
// them must be a valid request against the real SDK types.
func TestBenchmarkTasksAreValidTurboCalls(t *testing.T) {
	raw, err := os.ReadFile("../../bench/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	text := regexp.MustCompile(`"\{\{#\w+\}\}"`).ReplaceAllString(string(raw), "1760000000000")
	text = strings.ReplaceAll(text, "{{START_ISO}}", "2026-10-18T00:00:00Z")
	text = regexp.MustCompile(`\{\{\w+\}\}`).ReplaceAllString(text, "us-west-2")
	var doc struct {
		Tasks []struct {
			ID    string
			Turbo []struct {
				Tool      string
				Arguments json.RawMessage
			}
			Official []struct{ Tool string }
		}
	}
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Tasks) != 10 {
		t.Fatalf("want 10 use cases, have %d", len(doc.Tasks))
	}
	e := NewAWSEngine(aws.Config{Region: "us-west-2"})
	for _, task := range doc.Tasks {
		if len(task.Turbo) == 0 || len(task.Official) == 0 {
			t.Errorf("%s: both servers need a reference call", task.ID)
		}
		for _, call := range task.Turbo {
			var q Query
			if err := decode(call.Arguments, &q); err != nil || call.Tool != "aws_query" {
				t.Errorf("%s: %v", task.ID, err)
				continue
			}
			if q.Macro != "" {
				if _, ok := macroCatalog()[q.Macro]; !ok {
					t.Errorf("%s: unknown macro %s", task.ID, q.Macro)
				}
				continue
			}
			a, err := e.find(q.Action)
			if err != nil || a.Write {
				t.Errorf("%s: %v", task.ID, err)
				continue
			}
			if err := a.Validate(q.Params); err != nil {
				t.Errorf("%s: %s params: %v", task.ID, q.Action, err)
			}
			if q.Fields != "" {
				if _, _, err := fieldsProjection(a.root, strings.Split(q.Fields, ",")); err != nil {
					t.Errorf("%s: %v", task.ID, err)
				}
			}
		}
	}
}
