package turbo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
)

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func stubConfig(rt func(*http.Request) (*http.Response, error)) aws.Config {
	return aws.Config{Region: "us-west-1", Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	}), HTTPClient: &http.Client{Transport: transportFunc(rt)}}
}

func TestCompactFormatHoistsSharedValues(t *testing.T) {
	var rows []any
	for i, name := range []string{"api", "worker", "cron"} {
		rows = append(rows, map[string]any{
			"Name": name, "Runtime": "python3.13", "State": "Active",
			"Arn":      "arn:aws:lambda:us-west-1:123456789012:function:" + name,
			"Modified": fmt.Sprintf("2026-10-1%dT09:41:12.000+0000", i+1),
		})
	}
	got, err := Format(rows, "", 4096)
	if err != nil {
		t.Fatal(err)
	}
	want := "all: Runtime=python3.13 State=Active\n" +
		"prefix: Arn=arn:aws:lambda:us-west-1:123456789012:function:\n" +
		"Name\tArn\tModified\n" +
		"api\tapi\t2026-10-11 09:41Z\n" +
		"worker\tworker\t2026-10-12 09:41Z\n" +
		"cron\tcron\t2026-10-13 09:41Z"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if single, _ := Format(map[string]any{"Name": "api", "Timeout": 30.0, "Env": map[string]any{"A": "1"}}, "", 4096); single != "Name=api\nEnv={\"A\":\"1\"}\nTimeout=30" {
		t.Fatalf("object: %q", single)
	}
}

func TestRenderTruncatesRowsInsteadOfFailing(t *testing.T) {
	var rows []any
	for i := 0; i < 100; i++ {
		rows = append(rows, map[string]any{"Id": fmt.Sprintf("i-%04d", i), "Zone": fmt.Sprintf("z%d", i%3)})
	}
	r, err := Render(rows, "", 200)
	if err != nil || !r.Truncated() || r.Shown == 0 || r.Total != 100 || len(r.Text) > 200 {
		t.Fatalf("%+v %v", r, err)
	}
	e := NewEngine(Registry{})
	a := fakeAction("x.List", "Items", func(context.Context, json.RawMessage) (any, error) { return map[string]any{"Items": rows}, nil })
	e.Registry[a.Name] = a
	e.MaxBytes = 1024
	got, err := e.Query(context.Background(), "s", Query{Action: a.Name})
	if err != nil || !strings.Contains(got, "truncated: ") || len(got) > 1024 {
		t.Fatalf("%q %v", got, err)
	}
}

func TestFieldsGroupAndAutoPagination(t *testing.T) {
	var bodies []string
	var mu sync.Mutex
	cfg := stubConfig(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		if strings.Contains(string(b), `"nextToken":"p2"`) {
			return jsonResponse(`{"logGroups":[{"logGroupName":"/c","logGroupClass":"STANDARD","retentionInDays":7}]}`), nil
		}
		return jsonResponse(`{"logGroups":[{"logGroupName":"/a","logGroupClass":"STANDARD"},{"logGroupName":"/b","logGroupClass":"INFREQUENT_ACCESS"}],"nextToken":"p2"}`), nil
	})
	e := NewAWSEngine(cfg)
	e.reads = nil
	got, err := e.Query(context.Background(), "s", Query{Action: "logs.DescribeLogGroups", Fields: "LogGroupName"})
	if err != nil || got != "LogGroupName\n/a\n/b\nnext_params={\"NextToken\":\"p2\"}" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = e.Query(context.Background(), "s", Query{Action: "logs.DescribeLogGroups", Fields: "LogGroupName,LogGroupClass", All: true})
	if err != nil || !strings.Contains(got, "/c") || !strings.Contains(got, "pages=2") || strings.Contains(got, "next_params") {
		t.Fatalf("all pages: %q %v", got, err)
	}
	got, err = e.Query(context.Background(), "s", Query{Action: "logs.DescribeLogGroups", Fields: "LogGroupClass", Group: "LogGroupClass", All: true})
	if err != nil || !strings.HasPrefix(got, "LogGroupClass\tCount\nSTANDARD\t2\nINFREQUENT_ACCESS\t1") {
		t.Fatalf("group: %q %v", got, err)
	}
	if _, err := e.Query(context.Background(), "s", Query{Action: "logs.DescribeLogGroups", Fields: "Bad Name"}); err == nil {
		t.Fatal("accepted invalid field")
	}
	if _, err := e.Query(context.Background(), "s", Query{Action: "logs.DescribeLogGroups", Fields: "A", Projection: "@"}); err == nil {
		t.Fatal("accepted fields and projection together")
	}
}

func TestBatchRunsReadsAndLabelsResults(t *testing.T) {
	var calls atomic.Int32
	mk := func(name, item string) *Action {
		return fakeAction(name, "Items", func(context.Context, json.RawMessage) (any, error) {
			calls.Add(1)
			return map[string]any{"Items": []any{item}}, nil
		})
	}
	a, b := mk("a.List", "alpha"), mk("b.List", "beta")
	bad := fakeAction("c.List", "Items", func(context.Context, json.RawMessage) (any, error) { return nil, fmt.Errorf("denied") })
	e := NewEngine(Registry{a.Name: a, b.Name: b, bad.Name: bad})
	got, err := e.Query(context.Background(), "s", Query{Queries: []Query{{Action: "a.List"}, {Action: "b.List"}, {Action: "c.List"}}})
	if err != nil || !strings.Contains(got, "### 1 a.List\nValue\nalpha") || !strings.Contains(got, "### 2 b.List") || !strings.Contains(got, "Error: c.List: denied") || !strings.Contains(got, "partial=true") {
		t.Fatalf("%q %v", got, err)
	}
	for _, q := range []Query{
		{Queries: make([]Query, 9)},
		{Queries: []Query{{}}},
		{Queries: []Query{{Action: "a.List", Queries: []Query{{Action: "a.List"}}}}},
		{Action: "a.List", Macro: "x"},
		{},
	} {
		if _, err := e.Query(context.Background(), "s", q); err == nil {
			t.Fatalf("accepted %+v", q)
		}
	}
}

func TestReadCacheAvoidsRepeatCalls(t *testing.T) {
	var calls atomic.Int32
	a := fakeAction("a.List", "Items", func(context.Context, json.RawMessage) (any, error) {
		calls.Add(1)
		return map[string]any{"Items": []any{"x"}}, nil
	})
	e := NewEngine(Registry{a.Name: a})
	for i := 0; i < 3; i++ {
		if _, err := e.Query(context.Background(), "s", Query{Action: a.Name}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.Query(context.Background(), "s", Query{Action: a.Name, Params: json.RawMessage(`{"Other":1}`)}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d, want 2", calls.Load())
	}
}

func TestRecipeFillsParametersAndRunsStepsInParallel(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]map[string]any{}
	reg := Registry{}
	for _, name := range []string{"rds.DescribeDBInstances", "cloudwatch.GetMetricStatistics", "rds.DescribeEvents"} {
		reg[name] = fakeAction(name, "@", func(_ context.Context, raw json.RawMessage) (any, error) {
			p, _ := parameters(raw)
			mu.Lock()
			seen[name+fmt.Sprint(len(seen))] = p
			mu.Unlock()
			return map[string]any{"ok": true}, nil
		})
	}
	e := NewEngine(reg)
	got, err := e.Query(context.Background(), "s", Query{Macro: "rds_status", Params: json.RawMessage(`{"DBInstanceIdentifier":"prod-db"}`)})
	if err != nil || !strings.Contains(got, "### Instance") || !strings.Contains(got, "### CPU") || !strings.Contains(got, "### Events") {
		t.Fatalf("%q %v", got, err)
	}
	var metric map[string]any
	for _, p := range seen {
		if p["MetricName"] == "CPUUtilization" {
			metric = p
		}
	}
	dims, _ := metric["Dimensions"].([]any)
	if metric == nil || len(dims) != 1 || dims[0].(map[string]any)["Value"] != "prod-db" || metric["StartTime"] == nil || metric["Period"] != float64(300) {
		t.Fatalf("metric params: %+v", metric)
	}
	for _, bad := range []string{`{}`, `{"DBInstanceIdentifier":"x","Extra":1}`} {
		if _, err := e.Query(context.Background(), "s", Query{Macro: "rds_status", Params: json.RawMessage(bad)}); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	// An optional parameter that is absent removes the dependent part of the request.
	p, _ := expand(map[string]any{"LookupAttributes": []any{map[string]any{"AttributeValue": "$Username"}}, "MaxResults": 5}, map[string]any{})
	if m := p.(map[string]any); len(m["LookupAttributes"].([]any)) != 0 || m["MaxResults"] != 5 {
		t.Fatalf("%+v", p)
	}
}

func TestEveryRecipeActionExistsWithItsParameters(t *testing.T) {
	s := NewServiceSet(aws.Config{Region: "us-west-1"})
	for name, r := range recipes {
		for _, step := range r.Steps {
			a, err := s.Resolve(step.Action)
			if err != nil || a.Write {
				t.Errorf("%s/%s: %v", name, step.Label, err)
				continue
			}
			for k := range step.Params {
				if _, ok := a.Parameters[k]; !ok {
					t.Errorf("%s/%s: %s has no parameter %s", name, step.Label, step.Action, k)
				}
			}
			if step.Projection != "" {
				if _, err := compileProjection(step.Projection); err != nil {
					t.Errorf("%s/%s: %v", name, step.Label, err)
				}
			}
		}
	}
}

func TestShortErrorKeepsCodeAndMessage(t *testing.T) {
	err := fmt.Errorf("operation error EC2: DescribeInstances, https response error StatusCode: 403, RequestID: abc, api error %w", &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "You are not authorized"})
	if got := ShortError(err); got != "UnauthorizedOperation: You are not authorized" {
		t.Fatal(got)
	}
	if got := ShortError(&smithy.GenericAPIError{Code: "ExpiredToken", Message: "expired"}); !strings.Contains(got, "aws sso login") {
		t.Fatal(got)
	}
}

func TestDiscoverSearchesAcrossServices(t *testing.T) {
	e := NewAWSEngine(aws.Config{Region: "us-west-1"})
	all, err := e.Discover("")
	if err != nil || !strings.Contains(all, `"services":[`) || len(all) > 3000 {
		t.Fatalf("service list %d bytes: %v", len(all), err)
	}
	var found struct {
		Actions []struct {
			Action string
			Write  bool
		}
		Total int
	}
	out, err := e.Discover("lambda function list")
	if err != nil || json.Unmarshal([]byte(out), &found) != nil || found.Total == 0 || found.Actions[0].Action != "lambda.ListFunctions" && !strings.Contains(out, "lambda.ListFunctions") {
		t.Fatalf("%s %v", out, err)
	}
	out, _ = e.Discover("ec2")
	json.Unmarshal([]byte(out), &found)
	if len(found.Actions) != 60 || found.Total < 300 || found.Actions[0].Write {
		t.Fatalf("ec2 listing: %d shown of %d, first=%+v", len(found.Actions), found.Total, found.Actions[0])
	}
	if len(out) > 6000 {
		t.Fatalf("service listing is %d bytes", len(out))
	}
}

func TestDynamoDBItemsRoundTrip(t *testing.T) {
	var requests []string
	cfg := stubConfig(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		requests = append(requests, string(b))
		return jsonResponse(`{"Items":[{"pk":{"S":"user#1"},"age":{"N":"41"},"tags":{"SS":["a","b"]},"profile":{"M":{"admin":{"BOOL":true}}}}],"Count":1,"LastEvaluatedKey":{"pk":{"S":"user#1"}}}`), nil
	})
	e := NewAWSEngine(cfg)
	e.reads = nil
	got, err := e.Query(context.Background(), "s", Query{Action: "dynamodb.Query", Params: json.RawMessage(
		`{"TableName":"users","KeyConditionExpression":"pk = :p","ExpressionAttributeValues":{":p":{"S":"user#1"},":n":{"N":5},":l":{"L":[{"S":"x"},{"NULL":true}]}},"Limit":10}`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"S":"user#1"`, `"N":"5"`, `"L":[{"S":"x"},{"NULL":true}]`, `"TableName":"users"`, `"Limit":10`} {
		if !strings.Contains(requests[0], want) {
			t.Errorf("request lacks %s: %s", want, requests[0])
		}
	}
	for _, want := range []string{"age\tpk\tprofile\ttags", "41\tuser#1\t{\"admin\":true}\t[\"a\",\"b\"]", "next_params="} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"Value"`) {
		t.Errorf("union wrapper leaked:\n%s", got)
	}
	// The continuation can be passed straight back.
	if !strings.Contains(got, `"ExclusiveStartKey":{"pk":{"S":"user#1"}}`) {
		t.Errorf("continuation token:\n%s", got)
	}
}

func TestGeneratedWritesStayBehindTheMutationGate(t *testing.T) {
	cfg := stubConfig(func(*http.Request) (*http.Response, error) {
		t.Error("AWS was contacted")
		return nil, fmt.Errorf("unexpected")
	})
	e := NewAWSEngine(cfg)
	var audit strings.Builder
	e.Mutations = &MutationGuard{Enabled: true, Audit: &audit}
	ctx := context.Background()
	got, err := e.Mutate(ctx, "s", Mutation{Action: "sqs.PurgeQueue", Params: json.RawMessage(`{"QueueUrl":"https://sqs.us-west-1.amazonaws.com/1/q"}`), Intent: "reset test queue", Region: "us-west-2"})
	if err != nil || !strings.Contains(got, "no AWS call made") || !strings.Contains(audit.String(), `"region":"us-west-2"`) {
		t.Fatalf("%q %v audit=%s", got, err, audit.String())
	}
	if _, err := e.Mutate(ctx, "s", Mutation{Action: "sqs.PurgeQueue", Params: json.RawMessage(`{"Queue":"x"}`), Intent: "x"}); err == nil {
		t.Fatal("unknown parameter accepted")
	}
	if _, err := e.Mutate(ctx, "s", Mutation{Action: "sqs.ListQueues", Intent: "x"}); err == nil {
		t.Fatal("read accepted by mutate")
	}
	if _, err := e.Query(ctx, "s", Query{Action: "sqs.PurgeQueue"}); err == nil {
		t.Fatal("write accepted by query")
	}
	for _, name := range []string{"sts.AssumeRole", "iam.CreateAccessKey", "kms.Decrypt"} {
		if _, err := e.Mutate(ctx, "s", Mutation{Action: name, Intent: "x"}); err == nil {
			t.Fatalf("%s is reachable", name)
		}
	}
}

func TestEC2FieldsAndGroupOverQueryProtocol(t *testing.T) {
	xml := `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>r</requestId><reservationSet>
<item><reservationId>r-1</reservationId><instancesSet>
<item><instanceId>i-1</instanceId><instanceType>t3.micro</instanceType><instanceState><code>16</code><name>running</name></instanceState></item>
<item><instanceId>i-2</instanceId><instanceType>t3.micro</instanceType><instanceState><code>80</code><name>stopped</name></instanceState></item>
</instancesSet></item>
<item><reservationId>r-2</reservationId><instancesSet>
<item><instanceId>i-3</instanceId><instanceType>m5.large</instanceType><instanceState><code>16</code><name>running</name></instanceState></item>
</instancesSet></item></reservationSet></DescribeInstancesResponse>`
	cfg := stubConfig(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/xml"}}, Body: io.NopCloser(strings.NewReader(xml))}, nil
	})
	e := NewAWSEngine(cfg)
	e.DisableReadCache()
	got, err := e.Query(context.Background(), "s", Query{Action: "ec2.DescribeInstances", Fields: "InstanceId,State.Name"})
	if err != nil || got != "InstanceId\tState.Name\ni-1\trunning\ni-2\tstopped\ni-3\trunning" {
		t.Fatalf("fields: %q %v", got, err)
	}
	got, err = e.Query(context.Background(), "s", Query{Action: "ec2.DescribeInstances", Fields: "State.Name", Group: "State.Name"})
	if err != nil || got != "State.Name\tCount\nrunning\t2\nstopped\t1" {
		t.Fatalf("group: %q %v", got, err)
	}
	got, err = e.Query(context.Background(), "s", Query{Action: "ec2.DescribeInstances"})
	if err != nil || !strings.Contains(got, "i-2\t\tstopped") && !strings.Contains(got, "i-2") {
		t.Fatalf("preset: %q %v", got, err)
	}
}
