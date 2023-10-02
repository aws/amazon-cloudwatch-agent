// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"
)

func mergeCommandsWin(args ...string) string {
	return strings.Join(args, ";")
}
func mergeCommands(args ...string) string {
	return strings.Join(args, "&&")
}
func CloneGitRepo(gitUrl string, branch string) string {
	command := fmt.Sprintf(
		"git clone %s ccwa && cd ccwa && git checkout %s && cd ..",
		gitUrl,
		branch)

	return command
}
func RemoveFolder(folderPath string) string {
	command := fmt.Sprintf(
		"rm -rf %s", folderPath)
	return command
}
func MakeBuild() string {
	//assuming you are running this right after CloneGitRepo
	command := mergeCommands(
		"cd ccwa",
		" make amazon-cloudwatch-agent-linux amazon-cloudwatch-agent-windows package-rpm package-deb package-win ",
		"cd ..",
	)
	return command
}
func UploadToS3(key string) string {
	command := mergeCommands(
		fmt.Sprintf("echo 'key: %s %s'",
			S3_INTEGRATION_BUCKET,
			key,
		),
		"cd ccwa",
		fmt.Sprintf("aws s3 cp build/bin s3://%s/%s --recursive",
			S3_INTEGRATION_BUCKET,
			key,
		),
		fmt.Sprintf("aws s3 cp build/bin/linux/amd64/amazon-cloudwatch-agent.rpm s3://%s/%s/amazon_linux/amd64/latest/amazon-cloudwatch-agent.rpm",
			S3_INTEGRATION_BUCKET,
			key,
		),
		fmt.Sprintf("aws s3 cp build/bin/linux/arm64/amazon-cloudwatch-agent.rpm s3://%s/%s/amazon_linux/arm64/latest/amazon-cloudwatch-agent.rpm",
			S3_INTEGRATION_BUCKET,
			key,
		),
	)
	return command
}
func CopyBinary(key string) string {
	command := mergeCommands(fmt.Sprintf(
		"aws s3 cp s3://%s/%s . --recursive",
		S3_INTEGRATION_BUCKET,
		key,
	),
	)
	return command
}

// Windows Commands
func MakeMSI() string {
	return mergeCommands(
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
		S3_INTEGRATION_BUCKET,
		key,
	)
}
func UploadMSI(key string) string {
	return fmt.Sprintf(
		"aws s3 cp buildMSI.zip s3://%s/%s/buildMSI.zip",
		S3_INTEGRATION_BUCKET,
		key,
	)
}

// /
func LoadWorkDirectory(os OS) string {
	switch os {
	case MACOS:
		return "cd ~"
	default:
		return "echo 'Already at work directory' "
	}
}

// MAC COMMANDS
func MakeMacBinary() string {
	return "make amazon-cloudwatch-agent-darwin package-darwin"
}
func CopyBinaryMac() string {
	return mergeCommands(
		"echo cw agent version $(cat CWAGENT_VERSION)",
		"cp -r build/bin/darwin/amd64/. /tmp/",
		"cp -r build/bin/darwin/arm64/. /tmp/arm64/",
		"cp build/bin/CWAGENT_VERSION /tmp/CWAGENT_VERSION")
}
func CreatePkgCopyDeps() string {
	return mergeCommands(
		"cd ~",
		fmt.Sprintf("git clone %s test", TEST_REPO),
		"cd test",
		"cp -r pkg/tools/. /tmp/",
		"cp -r pkg/tools/. /tmp/arm64/",
		"cd ..",
	)
}
func BuildAndUploadMac(key string) string {
	bucket := S3_INTEGRATION_BUCKET
	return mergeCommands(
		"cd /tmp/",
		"chmod +x create_pkg.sh",
		"chmod +x arm64/create_pkg.sh",
		fmt.Sprintf("./create_pkg.sh %s/%s \"nosha\" amd64", bucket, key),
		"cd arm64",
		fmt.Sprintf("./create_pkg.sh  %s/%s \"nosha\" arm64", bucket, key),
		"cd ~",
	)
}
