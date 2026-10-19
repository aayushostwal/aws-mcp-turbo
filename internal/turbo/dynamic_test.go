package turbo

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jmespath/go-jmespath"
)

func TestEveryOperationHasUsableDefaults(t *testing.T) {
	s := NewServiceSet(aws.Config{Region: "us-west-1"})
	start := time.Now()
	reads, writes, tokens := 0, 0, 0
	for _, name := range s.names {
		d := s.services[name]
		ops := d.operations()
		if len(ops) == 0 {
			t.Errorf("service %s has no operations", name)
		}
		for _, o := range ops {
			if o.write {
				writes++
			} else {
				reads++
			}
			proj := autoProjection(o.out)
			if _, err := jmespath.Compile(proj); err != nil {
				t.Errorf("%s: bad projection %q: %v", o.name, proj, err)
			}
			if to, ti := paginationFields(o.in, o.out); to != "" {
				tokens++
				if ti == "" {
					t.Errorf("%s: token without input", o.name)
				}
			}
		}
	}
	t.Logf("services=%d reads=%d writes=%d paginated=%d indexed in %s", len(s.names), reads, writes, tokens, time.Since(start))
	if reads < 1500 {
		t.Fatalf("suspiciously few read operations: %d", reads)
	}
}

func TestPresetsPointAtRealOperations(t *testing.T) {
	s := NewServiceSet(aws.Config{Region: "us-west-1"})
	for name, p := range presets {
		a, err := s.Resolve(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if a.Projection != p.Projection {
			t.Errorf("%s: preset not applied", name)
		}
		for _, r := range p.Required {
			if _, ok := a.Parameters[r]; !ok {
				t.Errorf("%s: required %s is not an input field", name, r)
			}
		}
	}
}

func TestClassificationAndDenyList(t *testing.T) {
	s := NewServiceSet(aws.Config{Region: "us-west-1"})
	for name, write := range map[string]bool{
		"ec2.DescribeInstances": false, "s3.ListObjectsV2": false, "ec2.StopInstances": true, "lambda.UpdateFunctionConfiguration": true,
		"iam.DeleteRole": true, "dynamodb.Scan": false, "logs.StartQuery": false, "logs.GetQueryResults": false, "sqs.PurgeQueue": true,
	} {
		a, err := s.Resolve(name)
		if err != nil || a.Write != write {
			t.Errorf("%s: write=%v err=%v", name, a != nil && a.Write, err)
		}
	}
	for _, name := range []string{"secretsmanager.GetSecretValue", "s3.GetObject", "sts.AssumeRole", "kms.Decrypt", "ec2.GetPasswordData", "ecr.GetAuthorizationToken"} {
		if _, err := s.Resolve(name); err == nil {
			t.Errorf("%s must not be callable", name)
		}
	}
	a, _ := s.Resolve("ssm.GetParameter")
	if err := a.Validate([]byte(`{"Name":"/x","WithDecryption":true}`)); err == nil || !strings.Contains(err.Error(), "WithDecryption") {
		t.Errorf("decryption not blocked: %v", err)
	}
	if err := a.Validate([]byte(`{"Name":"/x"}`)); err != nil {
		t.Errorf("plain read rejected: %v", err)
	}
}

func TestDefaultProjectionsShowWhatMatters(t *testing.T) {
	s := NewServiceSet(aws.Config{Region: "us-west-1"})
	for name, want := range map[string][]string{
		"iam.ListAttachedRolePolicies": {"AttachedPolicies[]", "PolicyName", "PolicyArn"},
		"cloudfront.ListDistributions": {"DistributionList.Items[]", "Id", "Status"},
		"cloudwatch.ListMetrics":       {"Metrics[]", "MetricName", "Namespace"},
		"tagging.GetResources":         {"ResourceTagMappingList[]", "ResourceARN"},
		"ecs.DescribeServices":         {"Services[]", "Status", "DesiredCount"},
		"ec2.DescribeSubnets":          {"Subnets[]", "SubnetId", "State"},
		"dynamodb.Scan":                {"Items"},
		"athena.ListWorkGroups":        {"WorkGroups[]", "Name", "State"},
	} {
		a, err := s.Resolve(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, frag := range want {
			if !strings.Contains(a.Projection, frag) {
				t.Errorf("%s: projection %q lacks %q", name, a.Projection, frag)
			}
		}
		if len(a.Projection) > 400 {
			t.Errorf("%s: projection is %d bytes", name, len(a.Projection))
		}
	}
	if a, _ := s.Resolve("dynamodb.Scan"); a.root != "Items[]" {
		t.Errorf("scan root %q", a.root)
	}
	if a, _ := s.Resolve("ec2.DescribeInstances"); a.root != "Reservations[].Instances[]" {
		t.Errorf("preset root %q", a.root)
	}
}
