package turbo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jmespath/go-jmespath"
)

func fakeAction(name, projection string, fn func(context.Context, json.RawMessage) (any, error)) *Action {
	return &Action{Name: name, Projection: projection, expression: jmespath.MustCompile(projection), Call: fn}
}

func TestProjectionAndPagination(t *testing.T) {
	a := fakeAction("ec2.DescribeInstances", "Reservations[].Instances[].{Id:InstanceId,State:State.Name}", func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"Reservations": []any{map[string]any{"Instances": []any{map[string]any{"InstanceId": "i-1", "State": map[string]any{"Name": "running"}, "Secret": "hidden"}}}}, "NextToken": "next"}, nil
	})
	a.TokenIn = "NextToken"
	a.TokenOut = "NextToken"
	e := NewEngine(Registry{a.Name: a})
	got, err := e.Query(context.Background(), "s", Query{Action: a.Name})
	if err != nil || !strings.Contains(got, "i-1") || strings.Contains(got, "hidden") || !strings.Contains(got, `next_params={"NextToken":"next"}`) {
		t.Fatalf("%q %v", got, err)
	}
	got, err = e.Query(context.Background(), "s", Query{Action: a.Name, Projection: "Reservations[].Instances[].Secret", Format: "json"})
	if err != nil || !strings.Contains(got, `["hidden"]`) {
		t.Fatalf("override: %q %v", got, err)
	}
	for _, q := range []Query{{Action: a.Name, Projection: "["}, {Action: a.Name, Format: "csv"}, {Action: "ec2.DeleteEverything"}, {Action: a.Name, Params: json.RawMessage(`null`)}} {
		if _, err := e.Query(context.Background(), "s", q); err == nil {
			t.Fatalf("accepted invalid request: %+v", q)
		}
	}
}

func TestReadAllowlistAndSDKValidation(t *testing.T) {
	r := AWSRegistry(aws.Config{Region: "us-east-1"})
	e := NewEngine(r)
	if _, err := e.Query(context.Background(), "s", Query{Action: "ec2.StopInstances"}); err == nil {
		t.Fatal("write accepted by query")
	}
	for _, p := range []string{`{"bucket":"x"}`, `{"Bucket":"x","Unknown":1}`, `{"Bucket":null}`, `{"Bucket":5}`, `{}`} {
		if err := r["s3.ListObjectsV2"].Validate(json.RawMessage(p)); err == nil {
			t.Fatalf("accepted %s", p)
		}
	}
	if err := r["ec2.DescribeInstances"].Validate(json.RawMessage(`{"Filters":[{"Name":"tag:Name","Values":["api"]}]}`)); err != nil {
		t.Fatal(err)
	}
	got, err := e.Discover("ec2.DescribeInstances")
	if err != nil || !strings.Contains(got, "Filters") {
		t.Fatalf("discovery %q %v", got, err)
	}
	for name, a := range r {
		if a.expression == nil || a.Call == nil || a.Validate == nil {
			t.Fatalf("incomplete action %s", name)
		}
	}
}

func TestDeltaIsolationAndFailureAtomicity(t *testing.T) {
	var calls []map[string]any
	a := fakeAction("logs.GetLogEvents", "Events[].{Time:Timestamp,Message:Message}", func(_ context.Context, p json.RawMessage) (any, error) {
		args, _ := parameters(p)
		calls = append(calls, args)
		message := "first"
		token := "a"
		if args["NextToken"] == "a" {
			message = "second"
			token = "b"
		}
		return map[string]any{"Events": []any{map[string]any{"Timestamp": float64(42), "Message": message}}, "NextForwardToken": token}, nil
	})
	a.TokenOut = "NextForwardToken"
	a.TokenIn = "NextToken"
	e := NewEngine(Registry{a.Name: a})
	q := Query{Action: a.Name, Params: json.RawMessage(`{"LogGroupName":"g","LogStreamName":"s"}`)}
	if _, err := e.Query(context.Background(), "one", q); err != nil {
		t.Fatal(err)
	}
	e.MaxBytes = 5
	if _, err := e.Query(context.Background(), "one", q); err == nil {
		t.Fatal("expected output limit")
	}
	e.MaxBytes = 32768
	got, err := e.Query(context.Background(), "one", q)
	if err != nil || !strings.Contains(got, "second") {
		t.Fatalf("same timestamp lost: %q %v", got, err)
	}
	if calls[1]["NextToken"] != "a" || calls[2]["NextToken"] != "a" || calls[2]["StartFromHead"] != true {
		t.Fatalf("cursor advanced on error: %+v", calls)
	}
	if _, err := e.Query(context.Background(), "two", q); err != nil {
		t.Fatal(err)
	}
	if calls[3]["NextToken"] != nil {
		t.Fatal("session cache leaked")
	}
	q.Params = json.RawMessage(`{"LogGroupName":"g","LogStreamName":"s","NextToken":"manual"}`)
	got, err = e.Query(context.Background(), "one", q)
	if err != nil || !strings.Contains(got, "next_params=") || calls[4]["NextToken"] != "manual" {
		t.Fatalf("explicit cursor: %q %v", got, err)
	}
}

func TestConcurrentDeltaPolls(t *testing.T) {
	a := fakeAction("logs.GetLogEvents", "Events", func(_ context.Context, p json.RawMessage) (any, error) {
		args, _ := parameters(p)
		n := 0
		if s, ok := args["NextToken"].(string); ok {
			fmt.Sscanf(s, "%d", &n)
		}
		return map[string]any{"Events": []any{fmt.Sprintf("event-%d", n)}, "NextForwardToken": fmt.Sprint(n + 1)}, nil
	})
	a.TokenOut = "NextForwardToken"
	e := NewEngine(Registry{a.Name: a})
	var wg sync.WaitGroup
	results := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := e.Query(context.Background(), "s", Query{Action: a.Name})
			if err != nil {
				t.Error(err)
			}
			results <- s
		}()
	}
	wg.Wait()
	close(results)
	seen := map[string]bool{}
	for result := range results {
		if seen[result] {
			t.Fatalf("duplicate poll: %s", result)
		}
		seen[result] = true
	}
}

func TestCacheBoundsAndExpiry(t *testing.T) {
	c := NewDeltaCache(2, time.Minute)
	now := time.Now()
	c.put("a", cursor{Token: "1", At: now}, now)
	c.put("b", cursor{Token: "2", At: now}, now.Add(time.Second))
	c.put("c", cursor{Token: "3", At: now}, now.Add(2*time.Second))
	if c.get("a", now).Token != "" || len(c.streams) != 2 {
		t.Fatal("LRU bound failed")
	}
	if c.get("c", now.Add(2*time.Minute)).Token != "" || len(c.streams) != 0 {
		t.Fatal("TTL failed")
	}
	for i := 0; i < 40; i++ {
		c.put("ring", cursor{Token: fmt.Sprint(i), Timestamp: float64(i), At: now}, now)
	}
	if got := c.get("ring", now); got.Token != "39" || got.Timestamp != 39 {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	unlock, _ := c.lock(ctx, "busy")
	defer unlock()
	cancel()
	if _, err := c.lock(ctx, "busy"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestQueryTimeout(t *testing.T) {
	a := fakeAction("test.Read", "@", func(ctx context.Context, _ json.RawMessage) (any, error) { <-ctx.Done(); return nil, ctx.Err() })
	e := NewEngine(Registry{a.Name: a})
	e.Timeout = time.Millisecond
	if _, err := e.Query(context.Background(), "s", Query{Action: a.Name}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
