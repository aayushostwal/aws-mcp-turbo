package turbo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
