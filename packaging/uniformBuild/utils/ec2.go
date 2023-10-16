package utils

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"time"
	"uniformBuild/common"
)

// EC2CreateInstanceAPI defines the interface for the RunInstances and CreateTags functions.
// We use this interface to test the functions using a mocked service.
type EC2CreateInstanceAPI interface {
	RunInstances(ctx context.Context,
		params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)

	CreateTags(ctx context.Context,
		params *ec2.CreateTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
}

// EC2StopInstancesAPI defines the interface for the StopInstances function.
// We use this interface to test the function using a mocked service.
type EC2StopInstancesAPI interface {
	StopInstances(ctx context.Context,
		params *ec2.StopInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
}

func MakeInstance(c context.Context, api EC2CreateInstanceAPI, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	return api.RunInstances(c, input)
}
func AssignInstanceProfile(client *ec2.Client, instance *Instance) error {
	_, err := client.AssociateIamInstanceProfile(context.TODO(), &ec2.AssociateIamInstanceProfileInput{
		IamInstanceProfile: &types.IamInstanceProfileSpecification{
			Arn: aws.String(common.BUILD_ARN),
		},
		InstanceId: instance.InstanceId,
	})
	if err != nil {
		fmt.Println("Got an error attaching iam profile")
		return err
	}
	fmt.Println("IAM Instance Profile successfully added to ", instance.Name)
	return nil
}
func CreateInstanceCmd(client *ec2.Client, image *types.Image, name string, os common.OS) Instance {
	// Create separate values if required.
	minMaxCount := int32(1)

	input := &ec2.RunInstancesInput{
		ImageId:      image.ImageId,
		InstanceType: common.OS_TO_INSTANCE_TYPES[os],
		MinCount:     &minMaxCount,
		MaxCount:     &minMaxCount,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(name),
					},
					{
						Key:   aws.String("BuildEnv"),
						Value: aws.String("true"),
					},
				},
			},
		},
	}
	result, err := MakeInstance(context.TODO(), client, input)
	if err != nil {
		fmt.Println("Got an error creating an instance:")
		fmt.Println(err)
		return Instance{}
	}

	fmt.Printf("Created tagged instance with ID %s | \033[1m %s \033[0m \n", *result.Instances[0].InstanceId, name)
	instance := Instance{
		result.Instances[0],
		name,
		os,
	}

	return instance
}
func StopInstance(c context.Context, api EC2StopInstancesAPI, input *ec2.StopInstancesInput) (*ec2.StopInstancesOutput, error) {
	resp, err := api.StopInstances(c, input)

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "DryRunOperation" {
		fmt.Println("User has permission to stop instances.")
		input.DryRun = aws.Bool(false)
		return api.StopInstances(c, input)
	}

	return resp, err
}

// This function is not used but since Terminate and stopped can be used interchanagebly it is left behind
func StopInstanceCmd(client *ec2.Client, instanceID string) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []string{
			instanceID,
		},
		DryRun: aws.Bool(true),
	}

	_, err := StopInstance(context.TODO(), client, input)
	if err != nil {
		fmt.Println("Got an error stopping the instance")
		return err
	}
	fmt.Println("Stopped instance with ID " + instanceID)
	return nil
}

func TerminateInstanceCmd(client *ec2.Client, instanceID string) error {

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{
			instanceID,
		},
		DryRun: aws.Bool(false),
	}

	_, err := client.TerminateInstances(context.TODO(), input)
	if err != nil {
		fmt.Println("Got an error terminating the instance")
		return err
	}
	fmt.Println("Terminated the instance with ID " + instanceID)
	return nil
}

func WaitUntilAgentIsOn(client *ec2.Client, instance *Instance) {
	// Get instance status
	input := &ec2.DescribeInstanceStatusInput{
		InstanceIds: []string{*instance.InstanceId},
	}

	for retryCount := 0; retryCount < 10; retryCount++ {
		fmt.Printf("Trying to connect to ec2 instance, try count : %d \n", retryCount)
		output, err := client.DescribeInstanceStatus(context.Background(), input)
		if err != nil {
			fmt.Println(err)
			return
		}
		if len(output.InstanceStatuses) == 0 {
			time.Sleep(1 * time.Minute)
			continue
		}
		// Print instance status
		for _, status := range output.InstanceStatuses {
			fmt.Printf("Instance %s is %s\n", aws.ToString(status.InstanceId), aws.ToString((*string)(&status.InstanceState.Name)))
			return
		}

	}
}
func GetInstanceFromID(client *ec2.Client, instanceID string) *types.Instance {
	// Search for instance by ID
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}
	output, err := client.DescribeInstances(context.Background(), input)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	// Return instance object
	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return nil
	}
	return &output.Reservations[0].Instances[0]

}
