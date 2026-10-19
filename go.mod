module github.com/aayushostwal/aws-mcp-turbo

go 1.27.2

require (
	github.com/aws/aws-sdk-go-v2 v1.47.3
	github.com/aws/aws-sdk-go-v2/config v1.33.6
	github.com/aws/aws-sdk-go-v2/service/accessanalyzer v1.57.4
	github.com/aws/aws-sdk-go-v2/service/acm v1.50.1
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.50.0
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.44.0
	github.com/aws/aws-sdk-go-v2/service/appconfig v1.54.4
	github.com/aws/aws-sdk-go-v2/service/appsync v1.62.4
	github.com/aws/aws-sdk-go-v2/service/athena v1.66.4
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.78.1
	github.com/aws/aws-sdk-go-v2/service/backup v1.67.0
	github.com/aws/aws-sdk-go-v2/service/batch v1.78.3
	github.com/aws/aws-sdk-go-v2/service/bedrock v1.73.1
	github.com/aws/aws-sdk-go-v2/service/budgets v1.53.1
	github.com/aws/aws-sdk-go-v2/service/cloudcontrol v1.38.4
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.81.1
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.75.0
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.65.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.73.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.88.1
	github.com/aws/aws-sdk-go-v2/service/codebuild v1.78.4
	github.com/aws/aws-sdk-go-v2/service/codecommit v1.43.4
	github.com/aws/aws-sdk-go-v2/service/codedeploy v1.45.4
	github.com/aws/aws-sdk-go-v2/service/codepipeline v1.55.4
	github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider v1.75.3
	github.com/aws/aws-sdk-go-v2/service/configservice v1.74.4
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.74.1
	github.com/aws/aws-sdk-go-v2/service/directconnect v1.55.1
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.69.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.336.1
	github.com/aws/aws-sdk-go-v2/service/ecr v1.66.1
	github.com/aws/aws-sdk-go-v2/service/ecs v1.99.1
	github.com/aws/aws-sdk-go-v2/service/efs v1.49.4
	github.com/aws/aws-sdk-go-v2/service/eks v1.101.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.62.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing v1.41.4
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.63.1
	github.com/aws/aws-sdk-go-v2/service/emr v1.70.4
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.55.0
	github.com/aws/aws-sdk-go-v2/service/firehose v1.52.4
	github.com/aws/aws-sdk-go-v2/service/fsx v1.74.4
	github.com/aws/aws-sdk-go-v2/service/glue v1.171.0
	github.com/aws/aws-sdk-go-v2/service/guardduty v1.100.0
	github.com/aws/aws-sdk-go-v2/service/health v1.50.1
	github.com/aws/aws-sdk-go-v2/service/iam v1.64.1
	github.com/aws/aws-sdk-go-v2/service/identitystore v1.49.1
	github.com/aws/aws-sdk-go-v2/service/inspector2 v1.61.3
	github.com/aws/aws-sdk-go-v2/service/kafka v1.65.4
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.56.4
	github.com/aws/aws-sdk-go-v2/service/kms v1.61.1
	github.com/aws/aws-sdk-go-v2/service/lakeformation v1.56.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.110.0
	github.com/aws/aws-sdk-go-v2/service/mq v1.45.4
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.85.1
	github.com/aws/aws-sdk-go-v2/service/organizations v1.60.1
	github.com/aws/aws-sdk-go-v2/service/pricing v1.49.4
	github.com/aws/aws-sdk-go-v2/service/ram v1.45.4
	github.com/aws/aws-sdk-go-v2/service/rds v1.129.1
	github.com/aws/aws-sdk-go-v2/service/redshift v1.71.4
	github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi v1.41.4
	github.com/aws/aws-sdk-go-v2/service/route53 v1.70.1
	github.com/aws/aws-sdk-go-v2/service/route53resolver v1.54.4
	github.com/aws/aws-sdk-go-v2/service/s3 v1.113.4
	github.com/aws/aws-sdk-go-v2/service/sagemaker v1.284.0
	github.com/aws/aws-sdk-go-v2/service/scheduler v1.25.4
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.50.1
	github.com/aws/aws-sdk-go-v2/service/securityhub v1.85.0
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.43.4
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.79.1
	github.com/aws/aws-sdk-go-v2/service/sfn v1.51.4
	github.com/aws/aws-sdk-go-v2/service/sns v1.47.2
	github.com/aws/aws-sdk-go-v2/service/sqs v1.52.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.78.1
	github.com/aws/aws-sdk-go-v2/service/ssoadmin v1.49.4
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.1
	github.com/aws/aws-sdk-go-v2/service/transfer v1.86.0
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.83.4
	github.com/aws/aws-sdk-go-v2/service/xray v1.45.4
	github.com/aws/smithy-go v1.28.5
	github.com/jmespath/go-jmespath v0.4.0
	github.com/modelcontextprotocol/go-sdk v1.8.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.22 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.6 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.1 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/time v0.15.0 // indirect
)
