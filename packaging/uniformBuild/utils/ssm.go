package utils

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/schollz/progressbar/v3"
	"strconv"
	"time"
	"uniformBuild/commands"
	"uniformBuild/common"
)

const POWERSHELL_SSM_DOCUMENT = "AWS-RunPowerShellScript"
const BASHSHELL_SSM_DOCUMENT = "AWS-RunShellScript"

func enforceCommentLimit(s string) string {
	const commentCharLimit = 100
	if len(s) > commentCharLimit {
		return s[:commentCharLimit]
	}
	return s
}
func RunCmdRemotely(ssmClient *ssm.Client, instance *Instance, command string, comment string) error {
	// Specify the input for sending the command
	timeout := int32(common.COMMAND_TRACKING_TIMEOUT.Seconds())
	var shellType *string
	var masterCommand string
	if instance.os == common.WINDOWS {
		shellType = aws.String(POWERSHELL_SSM_DOCUMENT)
	} else {
		shellType = aws.String(BASHSHELL_SSM_DOCUMENT)
	}
	masterCommand = commands.MergeCommands(
		instance.os,
		commands.InitEnvCmd(instance.os),
		command,
	)
	sendCommandInput := &ssm.SendCommandInput{
		DocumentName: shellType,
		InstanceIds:  []string{*instance.InstanceId},
		Parameters: map[string][]string{
			"commands": {
				masterCommand,
			},
			"workingDirectory": {"~"},
			"executionTimeout": {strconv.Itoa(int(timeout))},
		},
		OutputS3BucketName: aws.String(common.S3_INTEGRATION_BUCKET),
		OutputS3KeyPrefix:  aws.String(common.S3_INTEGRATION_BUCKET + "/logs/"),
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
	status := CheckCommandStatus(ssmClient, commandID, instance.name)
	if status == ssmtypes.CommandStatusTimedOut {
		fmt.Println("Command timed out!")
		return errors.New(fmt.Sprintf("Command timed-out after %f seconds", common.COMMAND_TRACKING_TIMEOUT.Seconds()))
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
	const STATUS_BAR_THROTTLE = 10 * time.Second
	bar := progressbar.NewOptions(common.COMMAND_TRACKING_COUNT,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionThrottle(STATUS_BAR_THROTTLE),
	)
	desc := ""
	defer func() {
		bar.Describe(fmt.Sprintf("[%s]: %s", instanceTitle, desc))
		bar.Finish()
	}()
	for i := 0; i < common.COMMAND_TRACKING_COUNT; i++ {
		bar.Add(1)
		cmd := GetCommandInfo(ssmClient, commandID)
		//fmt.Printf("Time: %s , Status: %s \n", time.Now().String(), cmd.Status)
		desc = string(cmd.Status)
		switch cmd.Status {

		case ssmtypes.CommandStatusPending:
			i--
			time.Sleep(common.COMMAND_TRACKING_INTERVAL)
			continue
		case ssmtypes.CommandStatusFailed:
			desc = fmt.Sprintf("\033[31m%s\033[0m", cmd.Status)
			return cmd.Status
		case ssmtypes.CommandStatusSuccess:
			desc = fmt.Sprintf("\033[32m%s\033[0m", cmd.Status)
			return cmd.Status
		}
		bar.Describe(fmt.Sprintf("[%s]: %s", instanceTitle, desc))
		time.Sleep(common.COMMAND_TRACKING_INTERVAL)
	}
	return ssmtypes.CommandStatusTimedOut

}
