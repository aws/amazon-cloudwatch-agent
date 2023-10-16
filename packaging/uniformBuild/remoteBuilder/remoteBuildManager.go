package remoteBuilder

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"strings"
	"uniformBuild/commands"
	"uniformBuild/common"
	"uniformBuild/utils"
)

// This is the main struct that is managing the build process
type RemoteBuildManager struct {
	ssmClient       *ssm.Client
	instanceManager *utils.InstanceManager
	s3Client        *s3.Client
}

/*
This function will create EC2 instances as a side effect
*/
func CreateRemoteBuildManager(instanceGuide map[string]common.OS, accountID string) *RemoteBuildManager {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil
	}
	//instance := *GetInstanceFromID(client, "i-09fc6fdc80cd713a4")
	rbm := RemoteBuildManager{}

	rbm.instanceManager = utils.CreateNewInstanceManager(cfg, instanceGuide)
	fmt.Println("New Instance Manager Created")
	rbm.instanceManager.GetSupportedAMIs(accountID)
	fmt.Println("About to create ec2 instances")
	err = rbm.instanceManager.CreateEC2InstancesBlocking()

	if err != nil {
		panic(err)
	}
	fmt.Println("Starting SSM Client")
	rbm.ssmClient = ssm.NewFromConfig(cfg)
	//RunCmdRemotely(rbm.ssmClient, rbm.instances["linux"], "export PATH=$PATH:/usr/local/go/bin")
	rbm.s3Client = s3.NewFromConfig(cfg)
	return &rbm
}

// This function runs a command on a specific instance
func (rbm *RemoteBuildManager) RunCommand(cmdPacket commands.CommandPacket, instanceName string, comment string) error {
	if _, ok := rbm.instanceManager.Instances[instanceName]; !ok { //check if instance exist
		return errors.New("Invalid Instance Name")
	}
	if err := rbm.instanceManager.InsertOSRequirement(instanceName, cmdPacket.TargetOS); err != nil { //check if os has right OS
		return err
	}
	if isAlreadyBuilt := rbm.fileExistsInS3(cmdPacket.OutputFile); isAlreadyBuilt {
		//check if this command was already ran
		fmt.Println("Found cache skipping build")
		return nil
	}
	return utils.RunCmdRemotely(rbm.ssmClient, rbm.instanceManager.Instances[instanceName], cmdPacket.Command, comment)
}

// This function Builds CWA on a specific instance( it must be a linux instance)
func (rbm *RemoteBuildManager) BuildCWAAgent(gitUrl string, branch string, commitHash string, instanceName string) error {
	buildCommand := commands.CreateCommandPacket(
		common.LINUX,
		commitHash,
		commands.CloneGitRepo(gitUrl, branch),
		commands.MakeBuild(),
		commands.UploadToS3(commitHash),
	)
	fmt.Println("Starting CWA Build")
	return rbm.RunCommand(buildCommand, instanceName, fmt.Sprintf("building CWA | %s | branch: %s | hash: %s",
		strings.Replace(gitUrl, "https://github.com/", "", 1), branch, commitHash))
}

// Windows
func (rbm *RemoteBuildManager) MakeMsiZip(instanceName string, commitHash string) error {
	cachedFile := fmt.Sprintf("%s/buildMSI.zip", commitHash)
	command := commands.CreateCommandPacket(
		common.LINUX,
		cachedFile,
		commands.CloneGitRepo(common.TEST_REPO, "main"),
		"cd ccwa",
		commands.CopyBinary(commitHash),
		"ls -a",
		"unzip windows/amd64/amazon-cloudwatch-agent.zip -d windows-agent",
		commands.MakeMSI(),
		"zip buildMSI.zip msi_dep/*",
		commands.UploadMSI(commitHash),
	)
	return rbm.RunCommand(command, instanceName, fmt.Sprintf("Making MSI zip file for %s", commitHash))
}
func (rbm *RemoteBuildManager) BuildMSI(instanceName string, bucketKey string, packageKey string) error {
	cachedFile := fmt.Sprintf("%s/amazon-cloudwatch-agent.msi", packageKey)
	commandPacket := commands.CreateCommandPacket(
		common.WINDOWS,
		cachedFile,
		commands.CopyMsi(bucketKey),
		"Expand-Archive buildMSI.zip -DestinationPat C:\\buildMSI -Force",
		"cd C:\\buildMSI\\msi_dep",
		fmt.Sprintf(".\\create_msi.ps1 \"nosha\" %s/%s", common.S3_INTEGRATION_BUCKET, packageKey),
	)
	return rbm.RunCommand(commandPacket, instanceName, fmt.Sprintf("Making MSI Build file for %s", packageKey))
}

// / MACOS ------------
func (rbm *RemoteBuildManager) MakeMacPkg(instanceName string, commitHash string) error {
	cachedFile := fmt.Sprintf("%s/amd64/amazon-cloudwatch-agent.pkg", commitHash)
	commandPacket := commands.CreateCommandPacket(
		common.MACOS,
		cachedFile,
		commands.CloneGitRepo(common.MAIN_REPO, "main"),
		"cd ccwa",
		commands.MakeMacBinary(),
		commands.CopyBinaryMac(),
		commands.CreatePkgCopyDeps(),
		commands.BuildAndUploadMac(commitHash),
	)
	return rbm.RunCommand(commandPacket, instanceName, "Making Mac pkg")
}
func (rbm *RemoteBuildManager) Close() error {
	return rbm.instanceManager.Close()
}

// CACHE COMMANDS
func (rbm *RemoteBuildManager) fileExistsInS3(targetFile string) bool {
	fmt.Printf("Checking for %s cache \n", targetFile)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(common.S3_INTEGRATION_BUCKET),
		Key:    aws.String(targetFile),
	}
	_, err := rbm.s3Client.HeadObject(context.TODO(), input)
	if err != nil {
		fmt.Printf("Object %s does not exist in bucket %s\n", targetFile, common.S3_INTEGRATION_BUCKET)
		fmt.Println(err)
		return false
	}
	fmt.Printf("Object %s exists in bucket %s\n", common.S3_INTEGRATION_BUCKET, targetFile)
	return true

}
