package turbo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAWSAdapterUsesSignedSDKRequest(t *testing.T) {
	called := false
	cfg := aws.Config{Region: "us-east-1", Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	}), HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		if !strings.Contains(req.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
			t.Error("request was not signed")
		}
		if req.Header.Get("X-Amz-Target") != "Logs_20140328.GetLogEvents" {
			t.Error(req.Header)
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"logGroupName":"g"`) {
			t.Error(string(body))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}}, Body: io.NopCloser(strings.NewReader(`{"events":[{"timestamp":42,"message":"hello"}],"nextForwardToken":"f/1","nextBackwardToken":"b/1"}`))}, nil
	})}}
	e := NewEngine(AWSRegistry(cfg))
	got, err := e.Query(context.Background(), "s", Query{Action: "logs.GetLogEvents", Params: json.RawMessage(`{"LogGroupName":"g","LogStreamName":"s"}`)})
	if err != nil || !called || !strings.Contains(got, "hello") || strings.Contains(got, "ResultMetadata") {
		t.Fatalf("%s %v", got, err)
	}
}

func TestAWSQueryRegionOverridesSDKEndpoint(t *testing.T) {
	var hosts []string
	cfg := aws.Config{Region: "us-west-1", Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	}), HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}}, Body: io.NopCloser(strings.NewReader(`{"logGroups":[{"logGroupName":"found"}]}`))}, nil
	})}}
	e := NewAWSEngine(cfg)
	for _, region := range []string{"", "ap-south-1", "us-west-1"} {
		got, err := e.Query(context.Background(), "s", Query{Action: "logs.DescribeLogGroups", Region: region})
		if err != nil || !strings.Contains(got, "found") {
			t.Fatalf("region %q: %q %v", region, got, err)
		}
	}
	want := []string{"logs.us-west-1.amazonaws.com", "logs.ap-south-1.amazonaws.com", "logs.us-west-1.amazonaws.com"}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("hosts %v, want %v", hosts, want)
	}
}
