package turbo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Each entry is a reviewed read operation. Service method names alone do not
// establish safety, required inputs, or pagination behavior.
func bindExtendedAWS(r Registry, cfg aws.Config) {
	identity := sts.NewFromConfig(cfg)
	users := iam.NewFromConfig(cfg)
	metrics := cloudwatch.NewFromConfig(cfg)
	db := rds.NewFromConfig(cfg)
	elb := elasticloadbalancingv2.NewFromConfig(cfg)
	scaling := autoscaling.NewFromConfig(cfg)
	kubernetes := eks.NewFromConfig(cfg)
	stacks := cloudformation.NewFromConfig(cfg)
	dns := route53.NewFromConfig(cfg)
	tables := dynamodb.NewFromConfig(cfg)
	queues := sqs.NewFromConfig(cfg)
	topics := sns.NewFromConfig(cfg)
	api := apigateway.NewFromConfig(cfg)
	apiV2 := apigatewayv2.NewFromConfig(cfg)
	eventBus := eventbridge.NewFromConfig(cfg)
	secrets := secretsmanager.NewFromConfig(cfg)
	parameters := ssm.NewFromConfig(cfg)
	keys := kms.NewFromConfig(cfg)
	trails := cloudtrail.NewFromConfig(cfg)
	backups := backup.NewFromConfig(cfg)
	images := ecr.NewFromConfig(cfg)
	cache := elasticache.NewFromConfig(cfg)
	certs := acm.NewFromConfig(cfg)
	org := organizations.NewFromConfig(cfg)
	models := bedrock.NewFromConfig(cfg)

	bind(r, Action{Name: "sts.GetCallerIdentity", Projection: "{Account:Account,Arn:Arn,UserId:UserId}"}, identity.GetCallerIdentity)
	bind(r, Action{Name: "iam.ListUsers", Projection: "Users[].{Name:UserName,Arn:Arn,Id:UserId,Created:CreateDate}", TokenOut: "Marker", TokenIn: "Marker"}, users.ListUsers)
	bind(r, Action{Name: "iam.ListRoles", Projection: "Roles[].{Name:RoleName,Arn:Arn,Id:RoleId,Created:CreateDate}", TokenOut: "Marker", TokenIn: "Marker"}, users.ListRoles)
	bind(r, Action{Name: "cloudwatch.DescribeAlarms", Projection: "{Metric:MetricAlarms[].{Name:AlarmName,State:StateValue,Reason:StateReason,Metric:MetricName,Namespace:Namespace},Composite:CompositeAlarms[].{Name:AlarmName,State:StateValue,Reason:StateReason}}", TokenOut: "NextToken", TokenIn: "NextToken"}, metrics.DescribeAlarms)
	bind(r, Action{Name: "cloudwatch.GetMetricStatistics", Required: []string{"Namespace", "MetricName", "StartTime", "EndTime", "Period"}, Projection: "Datapoints[].{Time:Timestamp,Average:Average,Sum:Sum,Min:Minimum,Max:Maximum,Count:SampleCount,Unit:Unit}"}, metrics.GetMetricStatistics)
	bind(r, Action{Name: "rds.DescribeDBInstances", Projection: "DBInstances[].{Id:DBInstanceIdentifier,Arn:DBInstanceArn,Engine:Engine,Status:DBInstanceStatus,Class:DBInstanceClass,Endpoint:Endpoint.Address,MultiAZ:MultiAZ}", TokenOut: "Marker", TokenIn: "Marker"}, db.DescribeDBInstances)
	bind(r, Action{Name: "rds.DescribeDBClusters", Projection: "DBClusters[].{Id:DBClusterIdentifier,Arn:DBClusterArn,Engine:Engine,Status:Status,Endpoint:Endpoint,Writer:DBClusterMembers[?IsClusterWriter].DBInstanceIdentifier}", TokenOut: "Marker", TokenIn: "Marker"}, db.DescribeDBClusters)
	bind(r, Action{Name: "elbv2.DescribeLoadBalancers", Projection: "LoadBalancers[].{Arn:LoadBalancerArn,Name:LoadBalancerName,Type:Type,State:State.Code,DNS:DNSName,Scheme:Scheme}", TokenOut: "NextMarker", TokenIn: "Marker"}, elb.DescribeLoadBalancers)
	bind(r, Action{Name: "elbv2.DescribeTargetGroups", Projection: "TargetGroups[].{Arn:TargetGroupArn,Name:TargetGroupName,Port:Port,Protocol:Protocol,Type:TargetType,Vpc:VpcId}", TokenOut: "NextMarker", TokenIn: "Marker"}, elb.DescribeTargetGroups)
	bind(r, Action{Name: "autoscaling.DescribeAutoScalingGroups", Projection: "AutoScalingGroups[].{Name:AutoScalingGroupName,Arn:AutoScalingGroupARN,Desired:DesiredCapacity,Min:MinSize,Max:MaxSize,Instances:Instances[].{Id:InstanceId,State:LifecycleState,Health:HealthStatus}}", TokenOut: "NextToken", TokenIn: "NextToken"}, scaling.DescribeAutoScalingGroups)
	bind(r, Action{Name: "eks.ListClusters", Projection: "Clusters", TokenOut: "NextToken", TokenIn: "NextToken"}, kubernetes.ListClusters)
	bind(r, Action{Name: "eks.DescribeCluster", Required: []string{"Name"}, Projection: "Cluster.{Name:Name,Arn:Arn,Status:Status,Version:Version,Endpoint:Endpoint}"}, kubernetes.DescribeCluster)
	bind(r, Action{Name: "cloudformation.ListStacks", Projection: "StackSummaries[].{Name:StackName,Id:StackId,Status:StackStatus,Reason:StackStatusReason,Updated:LastUpdatedTime}", TokenOut: "NextToken", TokenIn: "NextToken"}, stacks.ListStacks)
	bind(r, Action{Name: "cloudformation.DescribeStacks", Projection: "Stacks[].{Name:StackName,Id:StackId,Status:StackStatus,Reason:StackStatusReason,Updated:LastUpdatedTime}", TokenOut: "NextToken", TokenIn: "NextToken"}, stacks.DescribeStacks)
	bind(r, Action{Name: "route53.ListHostedZones", Projection: "HostedZones[].{Id:Id,Name:Name,Private:Config.PrivateZone,Records:ResourceRecordSetCount}", TokenOut: "NextMarker", TokenIn: "Marker"}, dns.ListHostedZones)
	bind(r, Action{Name: "dynamodb.ListTables", Projection: "TableNames", TokenOut: "LastEvaluatedTableName", TokenIn: "ExclusiveStartTableName"}, tables.ListTables)
	bind(r, Action{Name: "dynamodb.DescribeTable", Required: []string{"TableName"}, Projection: "Table.{Name:TableName,Arn:TableArn,Status:TableStatus,Items:ItemCount,Bytes:TableSizeBytes,Billing:BillingModeSummary.BillingMode}"}, tables.DescribeTable)
	bind(r, Action{Name: "sqs.ListQueues", Projection: "QueueUrls", TokenOut: "NextToken", TokenIn: "NextToken"}, queues.ListQueues)
	bind(r, Action{Name: "sns.ListTopics", Projection: "Topics[].TopicArn", TokenOut: "NextToken", TokenIn: "NextToken"}, topics.ListTopics)
	bind(r, Action{Name: "sns.ListSubscriptions", Projection: "Subscriptions[].{Arn:SubscriptionArn,Topic:TopicArn,Protocol:Protocol,Endpoint:Endpoint}", TokenOut: "NextToken", TokenIn: "NextToken"}, topics.ListSubscriptions)
	bind(r, Action{Name: "apigateway.GetRestApis", Projection: "Items[].{Id:Id,Name:Name,Description:Description,Created:CreatedDate}", TokenOut: "Position", TokenIn: "Position"}, api.GetRestApis)
	bind(r, Action{Name: "apigatewayv2.GetApis", Projection: "Items[].{Id:ApiId,Name:Name,Protocol:ProtocolType,Endpoint:ApiEndpoint}", TokenOut: "NextToken", TokenIn: "NextToken"}, apiV2.GetApis)
	bind(r, Action{Name: "events.ListRules", Projection: "Rules[].{Name:Name,Arn:Arn,State:State,Bus:EventBusName}", TokenOut: "NextToken", TokenIn: "NextToken"}, eventBus.ListRules)
	bind(r, Action{Name: "events.ListEventBuses", Projection: "EventBuses[].{Name:Name,Arn:Arn}", TokenOut: "NextToken", TokenIn: "NextToken"}, eventBus.ListEventBuses)
	bind(r, Action{Name: "secretsmanager.ListSecrets", Projection: "SecretList[].{Name:Name,Arn:ARN,LastChanged:LastChangedDate,Rotation:RotationEnabled}", TokenOut: "NextToken", TokenIn: "NextToken"}, secrets.ListSecrets)
	bind(r, Action{Name: "secretsmanager.DescribeSecret", Required: []string{"SecretId"}, Projection: "{Name:Name,Arn:ARN,LastChanged:LastChangedDate,Rotation:RotationEnabled,Deleted:DeletedDate}"}, secrets.DescribeSecret)
	bind(r, Action{Name: "ssm.DescribeInstanceInformation", Projection: "InstanceInformationList[].{Id:InstanceId,Status:PingStatus,Platform:PlatformName,Version:PlatformVersion,Agent:AgentVersion}", TokenOut: "NextToken", TokenIn: "NextToken"}, parameters.DescribeInstanceInformation)
	bind(r, Action{Name: "ssm.DescribeParameters", Projection: "Parameters[].{Name:Name,Type:Type,Tier:Tier,Modified:LastModifiedDate}", TokenOut: "NextToken", TokenIn: "NextToken"}, parameters.DescribeParameters)
	bind(r, Action{Name: "kms.ListKeys", Projection: "Keys[].{Id:KeyId,Arn:KeyArn}", TokenOut: "NextMarker", TokenIn: "Marker"}, keys.ListKeys)
	bind(r, Action{Name: "kms.DescribeKey", Required: []string{"KeyId"}, Projection: "KeyMetadata.{Id:KeyId,Arn:Arn,State:KeyState,Usage:KeyUsage,Manager:KeyManager}"}, keys.DescribeKey)
	bind(r, Action{Name: "cloudtrail.DescribeTrails", Projection: "TrailList[].{Name:Name,Arn:TrailARN,Bucket:S3BucketName,MultiRegion:IsMultiRegionTrail,Logging:LogFileValidationEnabled}"}, trails.DescribeTrails)
	bind(r, Action{Name: "cloudtrail.LookupEvents", Projection: "Events[].{Time:EventTime,Name:EventName,Source:EventSource,User:Username,Resources:Resources[].ResourceName}", TokenOut: "NextToken", TokenIn: "NextToken"}, trails.LookupEvents)
	bind(r, Action{Name: "backup.ListBackupVaults", Projection: "BackupVaultList[].{Name:BackupVaultName,Arn:BackupVaultArn,RecoveryPoints:NumberOfRecoveryPoints}", TokenOut: "NextToken", TokenIn: "NextToken"}, backups.ListBackupVaults)
	bind(r, Action{Name: "backup.ListBackupPlans", Projection: "BackupPlansList[].{Id:BackupPlanId,Name:BackupPlanName,Version:VersionId}", TokenOut: "NextToken", TokenIn: "NextToken"}, backups.ListBackupPlans)
	bind(r, Action{Name: "ecr.DescribeRepositories", Projection: "Repositories[].{Name:RepositoryName,Arn:RepositoryArn,Uri:RepositoryUri,Created:CreatedAt}", TokenOut: "NextToken", TokenIn: "NextToken"}, images.DescribeRepositories)
	bind(r, Action{Name: "ecr.DescribeImages", Required: []string{"RepositoryName"}, Projection: "ImageDetails[].{Tags:ImageTags,Digest:ImageDigest,Pushed:ImagePushedAt,Bytes:ImageSizeInBytes}", TokenOut: "NextToken", TokenIn: "NextToken"}, images.DescribeImages)
	bind(r, Action{Name: "elasticache.DescribeCacheClusters", Projection: "CacheClusters[].{Id:CacheClusterId,Status:CacheClusterStatus,Engine:Engine,Version:EngineVersion,Nodes:NumCacheNodes}", TokenOut: "Marker", TokenIn: "Marker"}, cache.DescribeCacheClusters)
	bind(r, Action{Name: "elasticache.DescribeReplicationGroups", Projection: "ReplicationGroups[].{Id:ReplicationGroupId,Status:Status,Description:Description,NodeGroups:NodeGroups[].NodeGroupId}", TokenOut: "Marker", TokenIn: "Marker"}, cache.DescribeReplicationGroups)
	bind(r, Action{Name: "acm.ListCertificates", Projection: "CertificateSummaryList[].{Arn:CertificateArn,Domain:DomainName,Status:Status,Type:Type}", TokenOut: "NextToken", TokenIn: "NextToken"}, certs.ListCertificates)
	bind(r, Action{Name: "acm.DescribeCertificate", Required: []string{"CertificateArn"}, Projection: "Certificate.{Arn:CertificateArn,Domain:DomainName,Status:Status,Issuer:Issuer,Expires:NotAfter}"}, certs.DescribeCertificate)
	bind(r, Action{Name: "organizations.ListAccounts", Projection: "Accounts[].{Id:Id,Name:Name,Status:Status,State:State}", TokenOut: "NextToken", TokenIn: "NextToken"}, org.ListAccounts)
	bind(r, Action{Name: "organizations.DescribeOrganization", Projection: "Organization.{Id:Id,Arn:Arn,FeatureSet:FeatureSet,ManagementAccountId:MasterAccountId}"}, org.DescribeOrganization)
	bind(r, Action{Name: "bedrock.ListFoundationModels", Projection: "ModelSummaries[].{Id:ModelId,Name:ModelName,Provider:ProviderName,Input:InputModalities,Output:OutputModalities}"}, models.ListFoundationModels)
	bind(r, Action{Name: "bedrock.ListCustomModels", Projection: "ModelSummaries[].{Name:ModelName,Arn:ModelArn,Base:BaseModelArn,Created:CreationTime}", TokenOut: "NextToken", TokenIn: "NextToken"}, models.ListCustomModels)
}
