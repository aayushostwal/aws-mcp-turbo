package turbo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/accessanalyzer"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/appconfig"
	"github.com/aws/aws-sdk-go-v2/service/appsync"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/budgets"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/directconnect"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/emr"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	"github.com/aws/aws-sdk-go-v2/service/health"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/inspector2"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lakeformation"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/ram"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/securityhub"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/aws/aws-sdk-go-v2/service/xray"
)

// serviceDefs lists every AWS service reachable through aws_query and aws_mutate.
// Names are the prefix of an action ("ec2.DescribeInstances"). A service costs
// nothing at startup: its client is built on first use and its operations are
// indexed on first search.
func serviceDefs() []*serviceDef {
	return []*serviceDef{
		def("ec2", func(c aws.Config) *ec2.Client { return ec2.NewFromConfig(c) }),
		def("s3", func(c aws.Config) *s3.Client { return s3.NewFromConfig(c) }),
		def("lambda", func(c aws.Config) *lambda.Client { return lambda.NewFromConfig(c) }),
		def("ecs", func(c aws.Config) *ecs.Client { return ecs.NewFromConfig(c) }),
		def("logs", func(c aws.Config) *cloudwatchlogs.Client { return cloudwatchlogs.NewFromConfig(c) }),
		def("sts", func(c aws.Config) *sts.Client { return sts.NewFromConfig(c) }),
		def("iam", func(c aws.Config) *iam.Client { return iam.NewFromConfig(c) }),
		def("cloudwatch", func(c aws.Config) *cloudwatch.Client { return cloudwatch.NewFromConfig(c) }),
		def("rds", func(c aws.Config) *rds.Client { return rds.NewFromConfig(c) }),
		def("elbv2", func(c aws.Config) *elasticloadbalancingv2.Client { return elasticloadbalancingv2.NewFromConfig(c) }),
		def("elb", func(c aws.Config) *elasticloadbalancing.Client { return elasticloadbalancing.NewFromConfig(c) }),
		def("autoscaling", func(c aws.Config) *autoscaling.Client { return autoscaling.NewFromConfig(c) }),
		def("eks", func(c aws.Config) *eks.Client { return eks.NewFromConfig(c) }),
		def("cloudformation", func(c aws.Config) *cloudformation.Client { return cloudformation.NewFromConfig(c) }),
		def("route53", func(c aws.Config) *route53.Client { return route53.NewFromConfig(c) }),
		def("dynamodb", func(c aws.Config) *dynamodb.Client { return dynamodb.NewFromConfig(c) }),
		def("sqs", func(c aws.Config) *sqs.Client { return sqs.NewFromConfig(c) }),
		def("sns", func(c aws.Config) *sns.Client { return sns.NewFromConfig(c) }),
		def("apigateway", func(c aws.Config) *apigateway.Client { return apigateway.NewFromConfig(c) }),
		def("apigatewayv2", func(c aws.Config) *apigatewayv2.Client { return apigatewayv2.NewFromConfig(c) }),
		def("events", func(c aws.Config) *eventbridge.Client { return eventbridge.NewFromConfig(c) }),
		def("secretsmanager", func(c aws.Config) *secretsmanager.Client { return secretsmanager.NewFromConfig(c) }),
		def("ssm", func(c aws.Config) *ssm.Client { return ssm.NewFromConfig(c) }),
		def("kms", func(c aws.Config) *kms.Client { return kms.NewFromConfig(c) }),
		def("cloudtrail", func(c aws.Config) *cloudtrail.Client { return cloudtrail.NewFromConfig(c) }),
		def("backup", func(c aws.Config) *backup.Client { return backup.NewFromConfig(c) }),
		def("ecr", func(c aws.Config) *ecr.Client { return ecr.NewFromConfig(c) }),
		def("elasticache", func(c aws.Config) *elasticache.Client { return elasticache.NewFromConfig(c) }),
		def("acm", func(c aws.Config) *acm.Client { return acm.NewFromConfig(c) }),
		def("organizations", func(c aws.Config) *organizations.Client { return organizations.NewFromConfig(c) }),
		def("bedrock", func(c aws.Config) *bedrock.Client { return bedrock.NewFromConfig(c) }),
		def("ce", func(c aws.Config) *costexplorer.Client { return costexplorer.NewFromConfig(c) }),
		def("budgets", func(c aws.Config) *budgets.Client { return budgets.NewFromConfig(c) }),
		def("athena", func(c aws.Config) *athena.Client { return athena.NewFromConfig(c) }),
		def("glue", func(c aws.Config) *glue.Client { return glue.NewFromConfig(c) }),
		def("kinesis", func(c aws.Config) *kinesis.Client { return kinesis.NewFromConfig(c) }),
		def("firehose", func(c aws.Config) *firehose.Client { return firehose.NewFromConfig(c) }),
		def("sfn", func(c aws.Config) *sfn.Client { return sfn.NewFromConfig(c) }),
		def("cloudfront", func(c aws.Config) *cloudfront.Client { return cloudfront.NewFromConfig(c) }),
		def("wafv2", func(c aws.Config) *wafv2.Client { return wafv2.NewFromConfig(c) }),
		def("guardduty", func(c aws.Config) *guardduty.Client { return guardduty.NewFromConfig(c) }),
		def("securityhub", func(c aws.Config) *securityhub.Client { return securityhub.NewFromConfig(c) }),
		def("config", func(c aws.Config) *configservice.Client { return configservice.NewFromConfig(c) }),
		def("sesv2", func(c aws.Config) *sesv2.Client { return sesv2.NewFromConfig(c) }),
		def("sagemaker", func(c aws.Config) *sagemaker.Client { return sagemaker.NewFromConfig(c) }),
		def("opensearch", func(c aws.Config) *opensearch.Client { return opensearch.NewFromConfig(c) }),
		def("redshift", func(c aws.Config) *redshift.Client { return redshift.NewFromConfig(c) }),
		def("efs", func(c aws.Config) *efs.Client { return efs.NewFromConfig(c) }),
		def("emr", func(c aws.Config) *emr.Client { return emr.NewFromConfig(c) }),
		def("codebuild", func(c aws.Config) *codebuild.Client { return codebuild.NewFromConfig(c) }),
		def("codepipeline", func(c aws.Config) *codepipeline.Client { return codepipeline.NewFromConfig(c) }),
		def("ssoadmin", func(c aws.Config) *ssoadmin.Client { return ssoadmin.NewFromConfig(c) }),
		def("identitystore", func(c aws.Config) *identitystore.Client { return identitystore.NewFromConfig(c) }),
		def("cognito", func(c aws.Config) *cognitoidentityprovider.Client { return cognitoidentityprovider.NewFromConfig(c) }),
		def("route53resolver", func(c aws.Config) *route53resolver.Client { return route53resolver.NewFromConfig(c) }),
		def("kafka", func(c aws.Config) *kafka.Client { return kafka.NewFromConfig(c) }),
		def("tagging", func(c aws.Config) *resourcegroupstaggingapi.Client { return resourcegroupstaggingapi.NewFromConfig(c) }),
		def("servicequotas", func(c aws.Config) *servicequotas.Client { return servicequotas.NewFromConfig(c) }),
		def("health", func(c aws.Config) *health.Client { return health.NewFromConfig(c) }),
		def("pricing", func(c aws.Config) *pricing.Client { return pricing.NewFromConfig(c) }),
		def("appconfig", func(c aws.Config) *appconfig.Client { return appconfig.NewFromConfig(c) }),
		def("scheduler", func(c aws.Config) *scheduler.Client { return scheduler.NewFromConfig(c) }),
		def("ram", func(c aws.Config) *ram.Client { return ram.NewFromConfig(c) }),
		def("cloudcontrol", func(c aws.Config) *cloudcontrol.Client { return cloudcontrol.NewFromConfig(c) }),
		def("batch", func(c aws.Config) *batch.Client { return batch.NewFromConfig(c) }),
		def("inspector2", func(c aws.Config) *inspector2.Client { return inspector2.NewFromConfig(c) }),
		def("accessanalyzer", func(c aws.Config) *accessanalyzer.Client { return accessanalyzer.NewFromConfig(c) }),
		def("mq", func(c aws.Config) *mq.Client { return mq.NewFromConfig(c) }),
		def("appsync", func(c aws.Config) *appsync.Client { return appsync.NewFromConfig(c) }),
		def("transfer", func(c aws.Config) *transfer.Client { return transfer.NewFromConfig(c) }),
		def("directconnect", func(c aws.Config) *directconnect.Client { return directconnect.NewFromConfig(c) }),
		def("fsx", func(c aws.Config) *fsx.Client { return fsx.NewFromConfig(c) }),
		def("codecommit", func(c aws.Config) *codecommit.Client { return codecommit.NewFromConfig(c) }),
		def("codedeploy", func(c aws.Config) *codedeploy.Client { return codedeploy.NewFromConfig(c) }),
		def("xray", func(c aws.Config) *xray.Client { return xray.NewFromConfig(c) }),
		def("lakeformation", func(c aws.Config) *lakeformation.Client { return lakeformation.NewFromConfig(c) }),
	}
}
