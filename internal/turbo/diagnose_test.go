package turbo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestECSDiagnosis(t *testing.T) {
	list := fakeAction("ecs.ListTasks", "TaskArns", func(_ context.Context, p json.RawMessage) (any, error) {
		args, _ := parameters(p)
		if args["DesiredStatus"] != "STOPPED" {
			t.Error(args)
		}
		return map[string]any{"TaskArns": []any{"task-1"}, "NextToken": "more"}, nil
	})
	describe := fakeAction("ecs.DescribeTasks", "@", func(_ context.Context, p json.RawMessage) (any, error) {
		if !strings.Contains(string(p), "task-1") {
			t.Error(string(p))
		}
		return map[string]any{"Tasks": []any{map[string]any{"Reason": "Essential container exited", "Exit": 137}}, "Failures": []any{map[string]any{"Reason": "MISSING"}}}, nil
	})
	e := NewEngine(Registry{list.Name: list, describe.Name: describe})
	got, err := e.Diagnose(context.Background(), "s", "ecs_task_stopped", json.RawMessage(`{"Cluster":"prod"}`))
	if err != nil || !strings.Contains(got, "137") || !strings.Contains(got, "MISSING") || !strings.Contains(got, "Partial evidence") {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := e.Diagnose(context.Background(), "s", "ecs_task_stopped", json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing cluster accepted")
	}
}

func TestLambdaDiagnosisPartialEvidence(t *testing.T) {
	config := fakeAction("lambda.GetFunctionConfiguration", "@", func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"Timeout": 30, "MemoryMB": 128}, nil
	})
	logs := fakeAction("logs.FilterLogEvents", "@", func(_ context.Context, p json.RawMessage) (any, error) {
		args, _ := parameters(p)
		if args["LogGroupName"] != "/aws/lambda/example" {
			t.Error(args)
		}
		if !strings.Contains(args["FilterPattern"].(string), "Status: timeout") {
			t.Error(args)
		}
		return nil, errors.New("AccessDenied")
	})
	e := NewEngine(Registry{config.Name: config, logs.Name: logs})
	got, err := e.Diagnose(context.Background(), "s", "lambda_timeout", json.RawMessage(`{"FunctionName":"arn:aws:lambda:us-east-1:123456789012:function:example:live"}`))
	if err != nil || !strings.Contains(got, "Unavailable") || !strings.Contains(got, "AccessDenied") || !strings.Contains(got, "128") {
		t.Fatalf("%s %v", got, err)
	}
}
