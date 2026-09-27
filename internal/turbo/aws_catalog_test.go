package turbo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/encoding/cbor"
)

func TestAWSRegistryCoversThirtyServicesWithBoundedDiscovery(t *testing.T) {
	r := AWSRegistry(aws.Config{Region: "us-west-1"})
	wantServices := strings.Fields("ec2 s3 lambda ecs logs sts iam cloudwatch rds elbv2 autoscaling eks cloudformation route53 dynamodb sqs sns apigateway apigatewayv2 events secretsmanager ssm kms cloudtrail backup ecr elasticache acm organizations bedrock")
	gotServices := map[string]bool{}
	for name, a := range r {
		gotServices[strings.SplitN(name, ".", 2)[0]] = true
		if a.Name != name || a.Projection == "" || a.expression == nil || a.Call == nil || a.Validate == nil {
			t.Errorf("incomplete action %q", name)
		}
	}
	if len(gotServices) != len(wantServices) {
		t.Fatalf("services=%d, want=%d: %v", len(gotServices), len(wantServices), gotServices)
	}
	for _, service := range wantServices {
		if !gotServices[service] {
			t.Errorf("missing service %s", service)
		}
	}
	if len(r) != 61 {
		t.Fatalf("actions=%d, want 61", len(r))
	}
	discovery, err := NewEngine(r).Discover("")
	if err != nil || len(discovery) > 32768 {
		t.Fatalf("discovery bytes=%d: %v", len(discovery), err)
	}
	var catalog struct{ Actions []json.RawMessage }
	if err := json.Unmarshal([]byte(discovery), &catalog); err != nil || len(catalog.Actions) != len(r) {
		t.Fatalf("invalid discovery: %v, action count=%d", err, len(catalog.Actions))
	}
}

func TestRequestedAWSReadsAreRegisteredWithTypedInputs(t *testing.T) {
	r := AWSRegistry(aws.Config{Region: "us-west-1"})
	cases := []struct{ name, input string }{
		{"sts.GetCallerIdentity", `{}`},
		{"iam.ListUsers", `{}`},
		{"cloudwatch.DescribeAlarms", `{}`},
		{"cloudwatch.GetMetricStatistics", `{"Namespace":"AWS/EC2","MetricName":"CPUUtilization","StartTime":"2026-09-27T00:00:00Z","EndTime":"2026-09-27T01:00:00Z","Period":300,"Statistics":["Average"]}`},
		{"rds.DescribeDBInstances", `{}`},
		{"logs.DescribeLogGroups", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := r.Find(tc.name)
			if err != nil || a.Write || a.Validate(json.RawMessage(tc.input)) != nil {
				t.Fatalf("invalid read binding: %v", err)
			}
		})
	}
	if err := r["cloudwatch.GetMetricStatistics"].Validate(json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing metric request fields were accepted")
	}
	if err := r["iam.ListUsers"].Validate(json.RawMessage(`{"DeleteUser":true}`)); err == nil {
		t.Fatal("unknown parameter was accepted")
	}
}

func TestNewProjectionsRetainUsefulFieldsWithoutSDKMetadata(t *testing.T) {
	r := AWSRegistry(aws.Config{Region: "us-west-1"})
	cases := []struct{ name, raw, want string }{
		{"sts.GetCallerIdentity", `{"Account":"123456789012","Arn":"arn:aws:sts::123456789012:assumed-role/ops/me","ResultMetadata":{"RequestId":"irrelevant"}}`, "123456789012"},
		{"iam.ListUsers", `{"Users":[{"UserName":"alice","Arn":"arn:aws:iam::123456789012:user/alice","Path":"/"}],"ResultMetadata":{"RequestId":"irrelevant"}}`, "alice"},
		{"cloudwatch.DescribeAlarms", `{"MetricAlarms":[{"AlarmName":"cpu-high","StateValue":"ALARM"}],"CompositeAlarms":[{"AlarmName":"service-health","StateValue":"OK"}]}`, "service-health"},
		{"cloudwatch.GetMetricStatistics", `{"Datapoints":[{"Timestamp":"2026-09-27T01:00:00Z","Average":42.5,"Unit":"Percent"}]}`, "42.5"},
		{"rds.DescribeDBInstances", `{"DBInstances":[{"DBInstanceIdentifier":"primary","DBInstanceStatus":"available","Engine":"postgres"}]}`, "primary"},
		{"logs.DescribeLogGroups", `{"LogGroups":[{"LogGroupName":"/aws/lambda/api","StoredBytes":100}]}`, "/aws/lambda/api"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw any
			if err := json.Unmarshal([]byte(tc.raw), &raw); err != nil {
				t.Fatal(err)
			}
			projected, err := r[tc.name].expression.Search(raw)
			if err != nil {
				t.Fatal(err)
			}
			out, err := Format(projected, "", 32768)
			if err != nil || !strings.Contains(out, tc.want) || strings.Contains(out, "ResultMetadata") {
				t.Fatalf("projection %q: %v", out, err)
			}
		})
	}
}

func TestNewActionsExposeCorrectNextPageParameter(t *testing.T) {
	r := AWSRegistry(aws.Config{Region: "us-west-1"})
	cases := []struct{ name, output, input string }{
		{"iam.ListUsers", "Marker", "Marker"},
		{"rds.DescribeDBInstances", "Marker", "Marker"},
		{"elbv2.DescribeLoadBalancers", "NextMarker", "Marker"},
		{"dynamodb.ListTables", "LastEvaluatedTableName", "ExclusiveStartTableName"},
		{"logs.DescribeLogGroups", "NextToken", "NextToken"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := *r[tc.name]
			if a.TokenOut != tc.output || a.TokenIn != tc.input {
				t.Fatalf("pagination mapping %s → %s", a.TokenOut, a.TokenIn)
			}
			a.Call = func(context.Context, json.RawMessage) (any, error) {
				return map[string]any{tc.output: "cursor"}, nil
			}
			out, err := NewEngine(Registry{tc.name: &a}).Query(context.Background(), "s", Query{Action: tc.name})
			if err != nil || !strings.Contains(out, `next_params={"`+tc.input+`":"cursor"}`) {
				t.Fatalf("pagination output %q: %v", out, err)
			}
		})
	}
}

func TestNewSDKClientsUseSignedReadRequests(t *testing.T) {
	cases := []struct{ name, input, operation, response, want string }{
		{"sts.GetCallerIdentity", `{}`, "GetCallerIdentity", `<GetCallerIdentityResponse><GetCallerIdentityResult><Account>123456789012</Account><Arn>arn:aws:sts::123456789012:assumed-role/ops/me</Arn></GetCallerIdentityResult></GetCallerIdentityResponse>`, "123456789012"},
		{"iam.ListUsers", `{}`, "ListUsers", `<ListUsersResponse><ListUsersResult><Users><member><UserName>alice</UserName></member></Users></ListUsersResult></ListUsersResponse>`, "alice"},
		{"cloudwatch.DescribeAlarms", `{}`, "DescribeAlarms", "", "cpu-high"},
		{"rds.DescribeDBInstances", `{}`, "DescribeDBInstances", `<DescribeDBInstancesResponse><DescribeDBInstancesResult><DBInstances><DBInstance><DBInstanceIdentifier>primary</DBInstanceIdentifier><DBInstanceStatus>available</DBInstanceStatus></DBInstance></DBInstances></DescribeDBInstancesResult></DescribeDBInstancesResponse>`, "primary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			cfg := aws.Config{Region: "us-west-1", Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
			}), HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				called = true
				if !strings.Contains(req.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
					t.Error("request was not signed")
				}
				body, err := io.ReadAll(req.Body)
				if err != nil || (tc.name != "cloudwatch.DescribeAlarms" && !strings.Contains(string(body), "Action="+tc.operation)) || (tc.name == "cloudwatch.DescribeAlarms" && len(body) == 0) {
					t.Errorf("unexpected %s request body: %s (%v)", tc.operation, body, err)
				}
				if tc.name == "cloudwatch.DescribeAlarms" {
					// CloudWatch's current SDK protocol uses CBOR, not Query XML.
					payload := cbor.Encode(cbor.Map{"MetricAlarms": cbor.List{cbor.Map{"AlarmName": cbor.String("cpu-high"), "StateValue": cbor.String("ALARM")}}})
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/cbor"}}, Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/xml"}}, Body: io.NopCloser(strings.NewReader(tc.response))}, nil
			})}}
			out, err := NewEngine(AWSRegistry(cfg)).Query(context.Background(), "s", Query{Action: tc.name, Params: json.RawMessage(tc.input)})
			if err != nil || !called || !strings.Contains(out, tc.want) {
				t.Fatalf("output %q, called=%v, err=%v", out, called, err)
			}
		})
	}
}
