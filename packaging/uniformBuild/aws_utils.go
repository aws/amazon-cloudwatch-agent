package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
	"github.com/schollz/progressbar/v3"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	INVALID_INSTANCE = errors.New("Invalid Instance")
	INVALID_OS       = errors.New("That OS is not in supported AMIs")
)

type OS string

const (
	LINUX   OS = "linux"
	WINDOWS OS = "windows"
	//DARWIN  OS = "darwin"
)

var SUPPORTED_OS = []OS{LINUX, WINDOWS} //go doesn't let me create a slice from enum so this is the solution
type InstanceManager struct {
	ec2Client     *ec2.Client
	instances     map[string]*types.Instance
	amis          map[OS]*types.Image
	instanceGuide map[string]OS
}

func CreateNewInstanceManager(cfg aws.Config, instanceGuide map[string]OS) *InstanceManager {
	return &InstanceManager{
		ec2Client:     ec2.NewFromConfig(cfg),
		instances:     make(map[string]*types.Instance),
		amis:          make(map[OS]*types.Image),
		instanceGuide: instanceGuide,
	}
}
func (imng *InstanceManager) GetAllAMIVersions(accountID string) []types.Image {
	//returns a sorted list by creation date
	filters := []types.Filter{
		//{
		//	Name:   aws.String("owner-id"),
		//	Values: []string{"self"},
		//},
		{
			Name:   aws.String("tag-key"),
			Values: []string{"build-env"},
		},
	}
	// Get the latest AMI made by your own user
	resp, err := imng.ec2Client.DescribeImages(context.TODO(), &ec2.DescribeImagesInput{
		Filters: filters,
	})
	if err != nil {
		fmt.Println(err)
		return nil
	}
	// Sort the images based on the CreationDate field in descending order
	sort.Slice(resp.Images, func(i, j int) bool {
		return parseTime(*resp.Images[i].CreationDate).After(*parseTime(*resp.Images[j].CreationDate))
	})
	return resp.Images
}
func (imng *InstanceManager) GetLatestAMIVersion(accountID string) *types.Image {
	amiList := imng.GetAllAMIVersions(accountID)
	if len(amiList) > 0 {
		fmt.Println("Latest AMI ID:", *amiList[0].ImageId)
		return &amiList[0]
	} else {
		fmt.Println("No AMIs found.")
		return nil
	}
}
func parseTime(value string) *time.Time {
	t, err := time.Parse("2006-01-02T15:04:05.999999999Z", value)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return &t
}

func (imng *InstanceManager) GetSupportedAMIs(accountID string) {
	//this populates the amis map
	latestAmis := imng.GetAllAMIVersions(accountID) //this is sorted by date
	fmt.Printf("Found %d possible AMIs \n", len(latestAmis))
	for _, os := range SUPPORTED_OS {
		for _, ami := range latestAmis {
			if strings.Contains(strings.ToLower(*ami.PlatformDetails), string(os)) {
				imng.amis[os] = &ami
				break
			}
		}
	}

}
func (imng *InstanceManager) CreateEC2InstancesBlocking() error {
	//check if all OSes are valid
	for _, osType := range imng.instanceGuide {
		if _, ok := imng.amis[osType]; !ok {
			return INVALID_OS
		}
	}
	//create instances
	for instanceName, osType := range imng.instanceGuide {
		image := imng.amis[osType]
		instance := CreateInstanceCmd(imng.ec2Client, image, instanceName)
		imng.instances[instanceName] = &instance
	}
	time.Sleep(1 * time.Minute) // on average an ec2 launches in 60-90 seconds
	var wg sync.WaitGroup
	for _, instance := range imng.instances {
		wg.Add(1)
		go func(targetInstance *types.Instance) {
			defer wg.Done()
			WaitUntilAgentIsOn(imng.ec2Client, targetInstance)
			err := AssignInstanceProfile(imng.ec2Client, targetInstance)
			if err != nil {
				fmt.Println(err)
				return
			}
			time.Sleep(30 * time.Second)
		}(instance)
	}
	wg.Wait()
	return nil
}
func (imng *InstanceManager) Close() error {
	for instanceName, instance := range imng.instances {
		fmt.Printf("Closed instance: %s - %s \n", instanceName, *instance.InstanceId)
		err := StopInstanceCmd(imng.ec2Client, *instance.InstanceId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (imng *InstanceManager) insertOSRequirement(instanceName string, targetOS OS) error {
	instanceOS, ok := imng.instanceGuide[instanceName]
	if !ok {
		b, _ := json.MarshalIndent(imng.instanceGuide, "", "  ")
		fmt.Printf("%s is not in %s \n", instanceName, b)
		return INVALID_INSTANCE
	}
	if instanceOS == targetOS {
		return nil
	}
	return errors.New(fmt.Sprintf("This Instance is not the required OS, got: %s, requied: %s ", instanceOS, targetOS))

}

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
func AssignInstanceProfile(client *ec2.Client, instance *types.Instance) error {
	_, err := client.AssociateIamInstanceProfile(context.TODO(), &ec2.AssociateIamInstanceProfileInput{
		IamInstanceProfile: &types.IamInstanceProfileSpecification{
			Arn: aws.String(BUILD_ARN),
		},
		InstanceId: instance.InstanceId,
	})
	if err != nil {
		fmt.Println("Got an error attaching iam profile")
		return err
	}
	fmt.Println("IAM Instance Profile successfully added")
	return nil
}
func CreateInstanceCmd(client *ec2.Client, image *types.Image, name string) types.Instance {
	// Create separate values if required.
	minMaxCount := int32(1)

	input := &ec2.RunInstancesInput{
		ImageId:      image.ImageId,
		InstanceType: types.InstanceTypeT2Large,
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
					//{
					//	Key:   aws.String("BuildEnv"),
					//	Value: aws.String("true"),
					//},
				},
			},
		},
		//IamInstanceProfile: &types.IamInstanceProfileSpecification{
		//	Arn: aws.String(BUILD_ARN),
		//},
	}
	result, err := MakeInstance(context.TODO(), client, input)
	if err != nil {
		fmt.Println("Got an error creating an instance:")
		fmt.Println(err)
		return types.Instance{}
	}

	fmt.Printf("Created tagged instance with ID %s | %s \n", *result.Instances[0].InstanceId, name)
	return result.Instances[0]
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
func enforceCommentLimit(s string) string {
	const commentCharLimit = 100
	if len(s) > commentCharLimit {
		return s[:commentCharLimit]
	}
	return s
}
func RunCmdRemotely(ssmClient *ssm.Client, instance *types.Instance, command string, comment string) error {
	// Specify the input for sending the command
	timeout := int32(COMMAND_TRACKING_TIMEOUT.Seconds())
	sendCommandInput := &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []string{*instance.InstanceId},
		Parameters: map[string][]string{
			"commands": {
				initEnvCmd(),
				command},
			"workingDirectory": {"~"},
			"executionTimeout": {strconv.Itoa(int(timeout))},
		},
		OutputS3BucketName: aws.String(S3_INTEGRATION_BUCKET),
		OutputS3KeyPrefix:  aws.String(S3_INTEGRATION_BUCKET + "/logs/"),
		TimeoutSeconds:     aws.Int32(timeout),

		Comment: aws.String(enforceCommentLimit(comment)),
	}
	// Run the script on the instance
	fmt.Println("Command sent!")
	output, err := ssmClient.SendCommand(context.Background(), sendCommandInput)
	if err != nil {
		return errors.New(fmt.Sprintf("Coudln't send command with ssm: %s", err))
	}
	// Wait for the command to complete
	commandID := *output.Command.CommandId
	fmt.Printf("Waiting for command{%s}{%s}'s response\n", commandID, comment)
	status := CheckCommandStatus(ssmClient, commandID, "linux")
	if status == ssmtypes.CommandStatusTimedOut {
		fmt.Println("Command timed out!")
		return errors.New(fmt.Sprintf("Command timed-out after %f seconds", COMMAND_TRACKING_TIMEOUT.Seconds()))
	}
	fmt.Println("Command finished executing")
	// Get the command output
	cmdOut, err := ssmClient.ListCommandInvocations(context.TODO(), &ssm.ListCommandInvocationsInput{
		CommandId: &commandID,
		Details:   true,
	})
	if err != nil {
		fmt.Println(err)
		return err
	}

	if cmdOut.CommandInvocations[0].Status == ssmtypes.CommandInvocationStatusFailed {
		fmt.Println(*cmdOut.CommandInvocations[0].CommandPlugins[0].Output)
		return errors.New("Failed to execute command")
	}
	return nil

}
func WaitUntilAgentIsOn(client *ec2.Client, instance *types.Instance) {
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
func GetCommandsList(ssmClient *ssm.Client) {
	// List all commands
	resp, err := ssmClient.ListCommands(context.TODO(), &ssm.ListCommandsInput{})
	if err != nil {
		fmt.Println(err)
		return
	}
	// Print the command IDs and statuses
	for _, command := range resp.Commands {
		fmt.Printf("%s: %s\n", *command.CommandId, command.Status)
	}
}
func GetCommandInfo(ssmClient *ssm.Client, commandID string) ssmtypes.Command {
	resp, err := ssmClient.ListCommands(context.TODO(), &ssm.ListCommandsInput{
		CommandId: &commandID,
	})
	if err != nil {
		fmt.Println(err)
		return ssmtypes.Command{}
	}
	return resp.Commands[0]
}
func CheckCommandStatus(ssmClient *ssm.Client, commandID string, instanceTitle string) ssmtypes.CommandStatus {
	const STATUS_BAR_THROTTLE = 10
	bar := progressbar.NewOptions(COMMAND_TRACKING_COUNT,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionThrottle(STATUS_BAR_THROTTLE*time.Second),
	)
	desc := ""
	defer func() {
		bar.Describe(fmt.Sprintf("[%s]: %s", instanceTitle, desc))
		bar.Finish()
	}()
	for i := 0; i < COMMAND_TRACKING_COUNT; i++ {
		bar.Add(1)
		cmd := GetCommandInfo(ssmClient, commandID)
		//fmt.Printf("Time: %s , Status: %s \n", time.Now().String(), cmd.Status)
		desc = string(cmd.Status)
		switch cmd.Status {

		case ssmtypes.CommandStatusPending:
			i--
			time.Sleep(COMMAND_TRACKING_INTERVAL)
			continue
		case ssmtypes.CommandStatusFailed:
			desc = fmt.Sprintf("\033[31m%s\033[0m", cmd.Status)
			return cmd.Status
		case ssmtypes.CommandStatusSuccess:
			desc = fmt.Sprintf("\033[32m%s\033[0m", cmd.Status)
			return cmd.Status
		}
		bar.Describe(fmt.Sprintf("[%s]: %s", instanceTitle, desc))
		time.Sleep(COMMAND_TRACKING_INTERVAL)
	}
	return ssmtypes.CommandStatusTimedOut

}
