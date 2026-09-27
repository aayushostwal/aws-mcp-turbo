package turbo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func AWSRegistry(cfg aws.Config) Registry {
	r := Registry{}
	e, s, l := ec2.NewFromConfig(cfg), s3.NewFromConfig(cfg), lambda.NewFromConfig(cfg)
	c, logs := ecs.NewFromConfig(cfg), cloudwatchlogs.NewFromConfig(cfg)
	bind(r, Action{Name: "ec2.DescribeInstances", Projection: "Reservations[].Instances[].{Id:InstanceId,State:State.Name,Type:InstanceType,PrivateIp:PrivateIpAddress,Name:Tags[?Key=='Name'].Value | [0]}", TokenOut: "NextToken", TokenIn: "NextToken"}, e.DescribeInstances)
	bind(r, Action{Name: "ec2.DescribeVolumes", Projection: "Volumes[].{Id:VolumeId,State:State,SizeGiB:Size,Type:VolumeType,Zone:AvailabilityZone,Instances:Attachments[].InstanceId}", TokenOut: "NextToken", TokenIn: "NextToken"}, e.DescribeVolumes)
	bind(r, Action{Name: "ec2.DescribeSecurityGroups", Projection: "SecurityGroups[].{Id:GroupId,Name:GroupName,Vpc:VpcId,Ingress:IpPermissions,Egress:IpPermissionsEgress}", TokenOut: "NextToken", TokenIn: "NextToken"}, e.DescribeSecurityGroups)
	bind(r, Action{Name: "s3.ListBuckets", Projection: "Buckets[].{Name:Name,Created:CreationDate}", TokenOut: "ContinuationToken", TokenIn: "ContinuationToken"}, s.ListBuckets)
	bind(r, Action{Name: "s3.ListObjectsV2", Required: []string{"Bucket"}, Projection: "Contents[].{Key:Key,Bytes:Size,Modified:LastModified,Class:StorageClass}", TokenOut: "NextContinuationToken", TokenIn: "ContinuationToken"}, s.ListObjectsV2)
	bind(r, Action{Name: "s3.GetBucketLocation", Required: []string{"Bucket"}, Projection: "{Location:LocationConstraint}"}, s.GetBucketLocation)
	bind(r, Action{Name: "lambda.ListFunctions", Projection: "Functions[].{Name:FunctionName,Runtime:Runtime,MemoryMB:MemorySize,Timeout:Timeout,Modified:LastModified}", TokenOut: "NextMarker", TokenIn: "Marker"}, l.ListFunctions)
	bind(r, Action{Name: "lambda.GetFunctionConfiguration", Required: []string{"FunctionName"}, Projection: "{Name:FunctionName,State:State,Reason:StateReason,Runtime:Runtime,Timeout:Timeout,MemoryMB:MemorySize,LastUpdate:LastUpdateStatus,UpdateReason:LastUpdateStatusReason}"}, l.GetFunctionConfiguration)
	bind(r, Action{Name: "ecs.ListTasks", Projection: "TaskArns", TokenOut: "NextToken", TokenIn: "NextToken"}, c.ListTasks)
	bind(r, Action{Name: "ecs.DescribeTasks", Required: []string{"Tasks"}, Projection: "{Tasks:Tasks[].{Arn:TaskArn,Status:LastStatus,StopCode:StopCode,Reason:StoppedReason,Containers:Containers[].{Name:Name,Exit:ExitCode,Reason:Reason}},Failures:Failures}"}, c.DescribeTasks)
	bind(r, Action{Name: "logs.GetLogEvents", Required: []string{"LogGroupName", "LogStreamName"}, Projection: "Events[].{Time:Timestamp,Message:Message}", TokenOut: "NextForwardToken", TokenIn: "NextToken"}, logs.GetLogEvents)
	bind(r, Action{Name: "logs.FilterLogEvents", Required: []string{"LogGroupName"}, Projection: "Events[].{Time:Timestamp,Stream:LogStreamName,Message:Message}", TokenOut: "NextToken", TokenIn: "NextToken"}, logs.FilterLogEvents)
	bind(r, Action{Name: "ec2.StartInstances", Write: true, Required: []string{"InstanceIds"}, Projection: "StartingInstances[].{Id:InstanceId,Previous:PreviousState.Name,Current:CurrentState.Name}"}, e.StartInstances)
	bind(r, Action{Name: "ec2.StopInstances", Write: true, Required: []string{"InstanceIds"}, Projection: "StoppingInstances[].{Id:InstanceId,Previous:PreviousState.Name,Current:CurrentState.Name}"}, e.StopInstances)
	bind(r, Action{Name: "lambda.UpdateFunctionConfiguration", Write: true, Required: []string{"FunctionName"}, Projection: "{Name:FunctionName,State:State,LastUpdate:LastUpdateStatus,Reason:LastUpdateStatusReason}"}, l.UpdateFunctionConfiguration)
	return r
}
