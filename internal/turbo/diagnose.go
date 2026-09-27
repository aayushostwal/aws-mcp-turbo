package turbo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func macroCatalog() map[string]any {
	return map[string]any{
		"ecs_task_stopped": map[string]any{"params": []string{"Cluster (required)", "Tasks (optional array; up to 100)"}, "description": "Inspect stop reasons, container exit codes and AWS failures; lists stopped tasks if Tasks omitted."},
		"lambda_timeout":   map[string]any{"params": []string{"FunctionName (required)", "LogGroupName (optional)", "StartTime (optional epoch milliseconds)"}, "description": "Fetch function configuration and recent timeout evidence (last hour by default)."},
	}
}

func (e *Engine) Diagnose(ctx context.Context, session, macro string, params json.RawMessage) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	switch macro {
	case "ecs_task_stopped":
		var p struct {
			Cluster string
			Tasks   []string
		}
		if err := decode(params, &p); err != nil {
			return "", err
		}
		if p.Cluster == "" {
			return "", fmt.Errorf("Cluster is required")
		}
		pagination := ""
		if len(p.Tasks) == 0 {
			a, _ := e.Registry.Find("ecs.ListTasks")
			raw, err := a.Call(ctx, mustJSON(map[string]any{"Cluster": p.Cluster, "DesiredStatus": "STOPPED", "MaxResults": 100}))
			if err != nil {
				return "", err
			}
			obj, _ := raw.(map[string]any)
			if arns, ok := obj["TaskArns"].([]any); ok {
				for _, arn := range arns {
					if s, ok := arn.(string); ok {
						p.Tasks = append(p.Tasks, s)
					}
				}
			}
			if token, ok := obj["NextToken"].(string); ok && token != "" {
				pagination = "\nPartial evidence: more stopped tasks exist. Continue ecs.ListTasks with " + string(mustJSON(map[string]any{"Cluster": p.Cluster, "DesiredStatus": "STOPPED", "NextToken": token}))
			}
		}
		if len(p.Tasks) == 0 {
			return "No stopped tasks found in this page." + pagination, nil
		}
		if len(p.Tasks) > 100 {
			return "", fmt.Errorf("Tasks must contain at most 100 entries")
		}
		out, err := e.Query(ctx, session, Query{Action: "ecs.DescribeTasks", Params: mustJSON(map[string]any{"Cluster": p.Cluster, "Tasks": p.Tasks})})
		if err != nil {
			return "", err
		}
		return bounded("ECS stop evidence (reported reasons; not a definitive root cause):\n"+out+pagination, e.MaxBytes)
	case "lambda_timeout":
		var p struct {
			FunctionName string
			LogGroupName string
			StartTime    *int64
		}
		if err := decode(params, &p); err != nil {
			return "", err
		}
		if p.FunctionName == "" {
			return "", fmt.Errorf("FunctionName is required")
		}
		if p.LogGroupName == "" {
			name := p.FunctionName
			if strings.HasPrefix(name, "arn:") {
				parts := strings.Split(name, ":")
				if len(parts) < 7 {
					return "", fmt.Errorf("invalid Lambda ARN")
				}
				name = parts[6]
			} else {
				name = strings.Split(name, ":")[0]
			}
			p.LogGroupName = "/aws/lambda/" + name
		}
		start := time.Now().Add(-time.Hour).UnixMilli()
		if p.StartTime != nil {
			start = *p.StartTime
		}
		type result struct {
			label, text string
			err         error
		}
		results := make(chan result, 2)
		go func() {
			s, err := e.Query(ctx, session, Query{Action: "lambda.GetFunctionConfiguration", Params: mustJSON(map[string]any{"FunctionName": p.FunctionName})})
			results <- result{"Configuration", s, err}
		}()
		go func() {
			s, err := e.Query(ctx, session, Query{Action: "logs.FilterLogEvents", Params: mustJSON(map[string]any{"LogGroupName": p.LogGroupName, "StartTime": start, "FilterPattern": `?"Task timed out" ?"Status: timeout"`, "Limit": 100})})
			results <- result{"Timeout logs", s, err}
		}()
		byLabel := map[string]result{}
		for i := 0; i < 2; i++ {
			r := <-results
			byLabel[r.label] = r
		}
		if byLabel["Configuration"].err != nil && byLabel["Timeout logs"].err != nil {
			return "", fmt.Errorf("configuration unavailable: %v; timeout logs unavailable: %v", byLabel["Configuration"].err, byLabel["Timeout logs"].err)
		}
		var out strings.Builder
		out.WriteString("Lambda timeout evidence (one log page; absence is not proof of health):\n")
		for _, label := range []string{"Configuration", "Timeout logs"} {
			r := byLabel[label]
			out.WriteString(label + ":\n")
			if r.err != nil {
				out.WriteString("Unavailable: " + r.err.Error())
			} else {
				out.WriteString(r.text)
			}
			out.WriteByte('\n')
		}
		return bounded(out.String(), e.MaxBytes)
	default:
		return "", fmt.Errorf("unknown macro; use aws_discover")
	}
}
