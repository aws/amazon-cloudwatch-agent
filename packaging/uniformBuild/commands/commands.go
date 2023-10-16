// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package commands

import (
	"fmt"
	"strings"
	"uniformBuild/common"
)

type CommandPacket struct {
	TargetOS   common.OS
	OutputFile string
	Command    string
}

func CreateCommandPacket(targetOS common.OS, OutputFile string, commands ...string) CommandPacket {
	return CommandPacket{

		targetOS,
		OutputFile,
		MergeCommands(targetOS, commands...),
	}
}
func MergeCommands(TargetOS common.OS, args ...string) string {
	sep := "&&"
	if TargetOS == common.WINDOWS {
		sep = ";"
	}
	return strings.Join(args, sep)
}
func MergeCommandsWin(args ...string) string {
	return strings.Join(args, ";")
}

//	func MergeCommands(args ...string) string {
//		return strings.Join(args, "&&")
//	}
func InitEnvCmd(os common.OS) string {
	switch os {
	case common.MACOS:
		return MergeCommands(
			common.MACOS,
			"source /etc/profile",
			LoadWorkDirectory(os),
			"echo 'ENV SET FOR MACOS'",
		)
	case common.WINDOWS:
		return MergeCommands(
			common.WINDOWS,
			"$wixToolsetBinPath = \";C:\\Program Files (x86)\\WiX Toolset v3.11\\bin;\"",
			"$env:PATH = $env:PATH + $wixToolsetBinPath",
			LoadWorkDirectory(os),
		)
	default:
		return MergeCommands(
			common.LINUX,
			"export GOENV=/root/.config/go/env",
			"export GOCACHE=/root/.cache/go-build",
			"export GOMODCACHE=/root/go/pkg/mod",
			"export PATH=$PATH:/usr/local/go/bin",
			LoadWorkDirectory(os),
		)
	}

}

func CloneGitRepo(gitUrl string, branch string) string {
	command := MergeCommands(common.LINUX,
		fmt.Sprintf("git clone %s ccwa", gitUrl),
		"cd ccwa",
		fmt.Sprintf("git checkout %s", branch),
		"cd ..",
	)
	return command
}
func RemoveFolder(folderPath string) string {
	command := fmt.Sprintf(
		"rm -rf %s", folderPath)
	return command
}
func MakeBuild() string {
	//assuming you are running this right after CloneGitRepo
	command := MergeCommands(
		common.LINUX,
		"cd ccwa",
		" make amazon-cloudwatch-agent-linux amazon-cloudwatch-agent-windows package-rpm package-deb package-win  GOMODCACHE=true",
		//"make build",
		"cd ..",
	)
	return command
}
func UploadToS3(key string) string {
	command := MergeCommands(
		common.LINUX,
		fmt.Sprintf("echo 'key: %s %s'",
			common.S3_INTEGRATION_BUCKET,
			key,
		),
		"cd ccwa",
		fmt.Sprintf("aws s3 cp build/bin s3://%s/%s --recursive",
			common.S3_INTEGRATION_BUCKET,
			key,
		),
		fmt.Sprintf("aws s3 cp build/bin/linux/amd64/amazon-cloudwatch-agent.rpm s3://%s/%s/amazon_linux/amd64/latest/amazon-cloudwatch-agent.rpm",
			common.S3_INTEGRATION_BUCKET,
			key,
		),
		fmt.Sprintf("aws s3 cp build/bin/linux/arm64/amazon-cloudwatch-agent.rpm s3://%s/%s/amazon_linux/arm64/latest/amazon-cloudwatch-agent.rpm",
			common.S3_INTEGRATION_BUCKET,
			key,
		),
	)
	return command
}
func CopyBinary(key string) string {
	command := MergeCommands(
		common.LINUX,
		fmt.Sprintf(
			"aws s3 cp s3://%s/%s . --recursive",
			common.S3_INTEGRATION_BUCKET,
			key,
		),
	)
	return command
}

// Windows Commands
func MakeMSI() string {
	return MergeCommands(
		common.LINUX,
		"export version=$(cat CWAGENT_VERSION)",
		"echo cw agent version $version",
		"mkdir msi_dep",
		"cp -r msi/tools/. msi_dep/",
		"cp -r windows-agent/amazon-cloudwatch-agent/. msi_dep/",
		"go run msi/tools/msiversion/msiversionconverter.go $version msi_dep/amazon-cloudwatch-agent.wxs '<version>'",
		"go run msi/tools/msiversion/msiversionconverter.go $version msi_dep/manifest.json __VERSION__",
	)
}
func CopyMsi(key string) string {
	return fmt.Sprintf(
		"aws s3 cp s3://%s/%s/buildMSI.zip .",
		common.S3_INTEGRATION_BUCKET,
		key,
	)
}
func UploadMSI(key string) string {
	return fmt.Sprintf(
		"aws s3 cp buildMSI.zip s3://%s/%s/buildMSI.zip",
		common.S3_INTEGRATION_BUCKET,
		key,
	)
}

// /
func LoadWorkDirectory(os common.OS) string {
	switch os {
	case common.MACOS:
		return "cd ~"
	default:
		return "echo 'Already at work directory' "
	}
}

func RetrieveGoModVendor(targetOS common.OS) string {
	return MergeCommands(targetOS,
		"cd ccwa",
		fmt.Sprintf("aws s3 cp %s . ", common.GO_MOD_CACHE_DIR),
		"unzip -q vendor.zip",
		"rm -rf vendor.zip",
		"go mod vendor",
		"cd ..",
	)
}

// MAC COMMANDS
func MakeMacBinary() string {
	return "make amazon-cloudwatch-agent-darwin package-darwin"
}
func CopyBinaryMac() string {
	return MergeCommands(
		common.MACOS,
		"echo cw agent version $(cat CWAGENT_VERSION)",
		"cp -r build/bin/darwin/amd64/. /tmp/",
		"cp -r build/bin/darwin/arm64/. /tmp/arm64/",
		"cp build/bin/CWAGENT_VERSION /tmp/CWAGENT_VERSION")
}
func CreatePkgCopyDeps() string {
	return MergeCommands(
		common.MACOS,
		"cd ~",
		fmt.Sprintf("git clone %s test", common.TEST_REPO),
		"cd test",
		"cp -r pkg/tools/. /tmp/",
		"cp -r pkg/tools/. /tmp/arm64/",
		"cd ..",
	)
}
func BuildAndUploadMac(key string) string {
	bucket := common.S3_INTEGRATION_BUCKET
	return MergeCommands(
		common.MACOS,
		"cd /tmp/",
		"chmod +x create_pkg.sh",
		"chmod +x arm64/create_pkg.sh",
		fmt.Sprintf("./create_pkg.sh %s/%s \"nosha\" amd64", bucket, key),
		"cd arm64",
		fmt.Sprintf("./create_pkg.sh  %s/%s \"nosha\" arm64", bucket, key),
		"cd ~",
	)
}
