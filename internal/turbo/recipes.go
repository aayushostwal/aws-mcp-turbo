package turbo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// A recipe is a set of independent reads that together answer one operational
// question. Steps run in parallel; parameter values written "$Name" are filled
// from the request and "@now-1h", "@ms-1h" and "@date-30d" are relative times.
type recipe struct {
	Description string
	Required    []string
	Optional    []string
	Steps       []recipeStep
}

type recipeStep struct {
	Label      string
	Action     string
	Params     map[string]any
	Projection string
}

func (r recipe) describeParams() []string {
	var out []string
	for _, p := range r.Required {
		out = append(out, p+" (required)")
	}
	for _, p := range r.Optional {
		out = append(out, p+" (optional)")
	}
	return out
}

func metricStep(label, namespace, metric, dimension, param string, stats ...string) recipeStep {
	return recipeStep{Label: label, Action: "cloudwatch.GetMetricStatistics", Params: map[string]any{
		"Namespace": namespace, "MetricName": metric, "Period": 300, "Statistics": stats,
		"Dimensions": []any{map[string]any{"Name": dimension, "Value": "$" + param}},
		"StartTime":  "@now-1h", "EndTime": "@now",
	}, Projection: "Datapoints[].{Time:Timestamp,Avg:Average,Max:Maximum,Sum:Sum}"}
}

var recipes = map[string]recipe{
	"alb_target_health": {
		Description: "Health of every target in a target group, with reasons for unhealthy ones.",
		Required:    []string{"TargetGroupArn"},
		Steps: []recipeStep{
			{Label: "Targets", Action: "elbv2.DescribeTargetHealth", Params: map[string]any{"TargetGroupArn": "$TargetGroupArn"},
				Projection: "TargetHealthDescriptions[].{Target:Target.Id,Port:Target.Port,State:TargetHealth.State,Reason:TargetHealth.Reason,Detail:TargetHealth.Description}"},
		},
	},
	"rds_status": {
		Description: "RDS instance state, CPU and connections for the last hour, and events for the last day.",
		Required:    []string{"DBInstanceIdentifier"},
		Steps: []recipeStep{
			{Label: "Instance", Action: "rds.DescribeDBInstances", Params: map[string]any{"DBInstanceIdentifier": "$DBInstanceIdentifier"}},
			metricStep("CPU", "AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", "DBInstanceIdentifier", "Average", "Maximum"),
			metricStep("Connections", "AWS/RDS", "DatabaseConnections", "DBInstanceIdentifier", "DBInstanceIdentifier", "Average", "Maximum"),
			{Label: "Events", Action: "rds.DescribeEvents", Params: map[string]any{"SourceIdentifier": "$DBInstanceIdentifier", "SourceType": "db-instance", "Duration": 1440},
				Projection: "Events[].{Time:Date,Message:Message}"},
		},
	},
	"lambda_errors": {
		Description: "Lambda configuration plus invocations, errors, throttles and duration for the last hour.",
		Required:    []string{"FunctionName"},
		Steps: []recipeStep{
			{Label: "Configuration", Action: "lambda.GetFunctionConfiguration", Params: map[string]any{"FunctionName": "$FunctionName"}},
			metricStep("Invocations", "AWS/Lambda", "Invocations", "FunctionName", "FunctionName", "Sum"),
			metricStep("Errors", "AWS/Lambda", "Errors", "FunctionName", "FunctionName", "Sum"),
			metricStep("Throttles", "AWS/Lambda", "Throttles", "FunctionName", "FunctionName", "Sum"),
			metricStep("Duration", "AWS/Lambda", "Duration", "FunctionName", "FunctionName", "Average", "Maximum"),
		},
	},
	"ec2_instance": {
		Description: "EC2 instance details, status checks with scheduled events, and CPU for the last hour.",
		Required:    []string{"InstanceId"},
		Steps: []recipeStep{
			{Label: "Instance", Action: "ec2.DescribeInstances", Params: map[string]any{"InstanceIds": []any{"$InstanceId"}}},
			{Label: "Status", Action: "ec2.DescribeInstanceStatus", Params: map[string]any{"InstanceIds": []any{"$InstanceId"}, "IncludeAllInstances": true},
				Projection: "InstanceStatuses[].{Id:InstanceId,State:InstanceState.Name,System:SystemStatus.Status,Instance:InstanceStatus.Status,Events:Events[].Code}"},
			metricStep("CPU", "AWS/EC2", "CPUUtilization", "InstanceId", "InstanceId", "Average", "Maximum"),
		},
	},
	"ecs_service_status": {
		Description: "ECS service counts, deployment rollout state and the latest service events.",
		Required:    []string{"Cluster", "Service"},
		Steps: []recipeStep{
			{Label: "Service", Action: "ecs.DescribeServices", Params: map[string]any{"Cluster": "$Cluster", "Services": []any{"$Service"}},
				Projection: "Services[].{Name:ServiceName,Status:Status,Desired:DesiredCount,Running:RunningCount,Pending:PendingCount,Deployments:Deployments[].{State:RolloutState,Running:RunningCount,Desired:DesiredCount,Reason:RolloutStateReason},Events:Events[0:5].Message}"},
		},
	},
	"cost_by_service": {
		Description: "Unblended cost per AWS service for the last 30 days.",
		Steps: []recipeStep{
			{Label: "Cost", Action: "ce.GetCostAndUsage", Params: map[string]any{
				"TimePeriod": map[string]any{"Start": "@date-30d", "End": "@date"}, "Granularity": "MONTHLY", "Metrics": []any{"UnblendedCost"},
				"GroupBy": []any{map[string]any{"Type": "DIMENSION", "Key": "SERVICE"}}},
				Projection: "ResultsByTime[].Groups[].{Service:Keys[0],USD:Metrics.UnblendedCost.Amount}"},
		},
	},
	"cloudtrail_recent": {
		Description: "API activity from the last hour, optionally for one user or role session name.",
		Optional:    []string{"Username"},
		Steps: []recipeStep{
			{Label: "Events", Action: "cloudtrail.LookupEvents", Params: map[string]any{
				"LookupAttributes": []any{map[string]any{"AttributeKey": "Username", "AttributeValue": "$Username"}}, "StartTime": "@now-1h", "MaxResults": 50}},
		},
	},
}

var relativeTime = map[string]func(time.Time, time.Duration) any{
	"now":  func(t time.Time, d time.Duration) any { return t.Add(d).UTC().Format(time.RFC3339) },
	"ms":   func(t time.Time, d time.Duration) any { return t.Add(d).UnixMilli() },
	"date": func(t time.Time, d time.Duration) any { return t.Add(d).UTC().Format("2006-01-02") },
}

// expand fills placeholders. ok is false when a value depends on a missing
// optional parameter, so that part of the request is left out.
func expand(v any, params map[string]any) (any, bool) {
	switch x := v.(type) {
	case string:
		if strings.HasPrefix(x, "$") {
			p, ok := params[x[1:]]
			return p, ok
		}
		if strings.HasPrefix(x, "@") {
			kind, offset, _ := strings.Cut(x[1:], "-")
			f, known := relativeTime[kind]
			if !known {
				return x, true
			}
			var d time.Duration
			if offset != "" {
				unit := offset[len(offset)-1]
				var n int
				fmt.Sscanf(offset[:len(offset)-1], "%d", &n)
				switch unit {
				case 'd':
					d = time.Duration(n) * 24 * time.Hour
				default:
					d, _ = time.ParseDuration(offset)
				}
			}
			return f(time.Now(), -d), true
		}
		return x, true
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			if v, ok := expand(e, params); ok {
				out = append(out, v)
			}
		}
		return out, true
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			v, ok := expand(e, params)
			if !ok {
				return nil, false
			}
			out[k] = v
		}
		return out, true
	}
	return v, true
}

func (e *Engine) runRecipe(ctx context.Context, session string, r recipe, raw json.RawMessage) (string, error) {
	params, err := parameters(raw)
	if err != nil {
		return "", err
	}
	allowed := map[string]bool{}
	for _, p := range append(append([]string{}, r.Required...), r.Optional...) {
		allowed[p] = true
	}
	for k := range params {
		if !allowed[k] {
			return "", fmt.Errorf("unknown parameter %q; expected %s", k, strings.Join(r.describeParams(), ", "))
		}
	}
	for _, k := range r.Required {
		if v, ok := params[k]; !ok || v == nil || v == "" {
			return "", fmt.Errorf("%s is required", k)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	share := e.MaxBytes / len(r.Steps)
	results := runParallel(ctx, len(r.Steps), func(i int) batchResult {
		step := r.Steps[i]
		p := map[string]any{}
		for k, v := range step.Params {
			if ev, ok := expand(v, params); ok {
				p[k] = ev
			}
		}
		text, err := e.withBudget(share, func(s *Engine) (string, error) {
			return s.queryOne(ctx, session, Query{Action: step.Action, Params: mustJSON(p), Projection: step.Projection})
		})
		return batchResult{label: step.Label, text: text, err: err}
	})
	return e.joinResults(results)
}
