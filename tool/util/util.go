// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	sysruntime "runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/interfaze"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/stdin"
)

const (
	configJsonFileName = "config.json"
	OsTypeLinux        = "linux"
	OsTypeWindows      = "windows"
	OsTypeDarwin       = "darwin"

	MapKeyMetricsCollectionInterval = "metrics_collection_interval"
	MapKeyInstances                 = "resources"
	MapKeyMeasurement               = "measurement"

	StandardLogGroupClass         = "STANDARD"
	InfrequentAccessLogGroupClass = "INFREQUENT_ACCESS"
)

func CurOS() string {
	return sysruntime.GOOS
}
func getBackupDir() string {
	switch sysruntime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Amazon", "CloudWatchAgent", "etc", "backup-configs")
	default:
		return "/opt/aws/amazon-cloudwatch-agent/etc/backup-configs"
	}
}
func FileBackup(filePath string, dirPath string) error {
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) || fileInfo.Size() == 0 {
		return nil
	} else if err = backupConfigFile(filePath, dirPath); err != nil {
		return err
	}
	return nil
}
func CurPath() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return path.Dir(ex)
}

func ConfigFilePath() string {
	return filepath.Join(CurPath(), configJsonFileName)
}

func PermissionCheck() {
	filePath := ConfigFilePath()
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		fmt.Printf("Make sure that you have write permission to %s\n", filePath)
		os.Exit(1)
	}
	defer f.Close()
	return
}

func ReadConfigFromJsonFile() string {
	filePath := ConfigFilePath()
	byteArray, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error in reading config from file %s: %v\n", filePath, err)
		os.Exit(1)
	}
	return string(byteArray)
}

func SerializeResultMapToJsonByteArray(resultMap map[string]interface{}) []byte {
	resultByteArray, err := json.MarshalIndent(resultMap, "", "\t")
	if err != nil {
		fmt.Printf("Result map to byte array json marshal error: %v\n", err)
		os.Exit(1)
	}
	return resultByteArray
}

func SaveResultByteArrayToJsonFile(resultByteArray []byte, filePath string) string {
	//make a backup of file if it exists
	dirPath := getBackupDir()
	err := FileBackup(filePath, dirPath)
	if err != nil {
		fmt.Println("There was an error trying to backup your file: ", err)
	} else {
		fmt.Println("Existing config JSON identified and copied to: ", dirPath)
	}
	err = os.WriteFile(filePath, resultByteArray, 0755)
	if err != nil {
		fmt.Printf("Error in writing file to %s: %v\nMake sure that you have write permission to %s.", filePath, err, filePath)
		os.Exit(1)
	}
	fmt.Printf("Saved config file to %s successfully.\n", filePath)
	return filePath
}
func backupConfigFile(configFilePath, backupDirPath string) error {

	err := os.MkdirAll(backupDirPath, 0755)
	if err != nil {
		return err
	}
	files, err := os.ReadDir(backupDirPath)
	if err != nil {
		return err
	}

	newBackupNumber := len(files) + 1
	if len(files) >= 10 {
		sort.Slice(files, func(i, j int) bool {
			infoI, _ := files[i].Info()
			infoJ, _ := files[j].Info()
			return infoI.ModTime().Before(infoJ.ModTime())
		})

		removedFileName := files[0].Name()
		removedNumberStr := strings.TrimSuffix(strings.TrimPrefix(removedFileName, "config-"), ".json")
		removedNumber, err := strconv.Atoi(removedNumberStr)
		if err != nil {
			return err
		}
		err = os.Remove(filepath.Join(backupDirPath, files[0].Name()))
		if err != nil {
			return err
		}
		newBackupNumber = removedNumber + 10
	}
	backupFilePath := filepath.Join(backupDirPath, fmt.Sprintf("config-%d.json", newBackupNumber))

	backUpFile, err := os.Create(backupFilePath)
	defer backUpFile.Close()
	if err != nil {
		return err
	}
	configFile, err := os.Open(configFilePath)
	defer configFile.Close()
	if err != nil {
		return err
	}
	//copying file to backup
	_, err = io.Copy(backUpFile, configFile)
	return err
}

func SDKRegion() (region string) {
	ses, err := session.NewSession()

	if err != nil {
		return
	}
	if ses.Config != nil && ses.Config.Region != nil {
		region = *ses.Config.Region
	}
	return region
}

func SDKRegionWithProfile(profile string) (region string) {
	ses, err := session.NewSessionWithOptions(session.Options{Profile: profile, SharedConfigState: session.SharedConfigEnable})

	if err != nil {
		return
	}
	if ses.Config != nil && ses.Config.Region != nil {
		region = *ses.Config.Region
	}
	return region
}

func SDKCredentials() (accessKey, secretKey string, creds *credentials.Credentials) {
	ses, err := session.NewSession()
	if err != nil {
		return
	}
	if ses.Config != nil && ses.Config.Credentials != nil {
		if credsValue, err := ses.Config.Credentials.Get(); err == nil {
			accessKey = credsValue.AccessKeyID
			secretKey = credsValue.SecretAccessKey
			creds = ses.Config.Credentials
		}
	}
	return
}

func DefaultEC2Region() (region string) {
	fmt.Println("Trying to fetch the default region based on ec2 metadata...")
	// imds should by the time user can run the wizard
	sesFallBackDisabled, err := session.NewSession(&aws.Config{
		LogLevel:                  configaws.SDKLogLevel(),
		Logger:                    configaws.SDKLogger{},
		EC2MetadataEnableFallback: aws.Bool(false),
		Retryer:                   retryer.NewIMDSRetryer(retryer.GetDefaultRetryNumber()),
	})
	sesFallBackEnabled, err := session.NewSession(&aws.Config{
		LogLevel: configaws.SDKLogLevel(),
		Logger:   configaws.SDKLogger{},
	})
	if err != nil {
		return
	}
	md := ec2metadata.New(sesFallBackDisabled)
	if info, errOuter := md.Region(); errOuter == nil {
		region = info
	} else {
		log.Printf("D! could not get region from imds v2 thus enable fallback")
		mdInner := ec2metadata.New(sesFallBackEnabled)
		if infoInner, errInner := mdInner.Region(); errInner == nil {
			region = infoInner
		} else {
			fmt.Printf("W! could not get region from ec2 metadata... %v", errInner)
		}
	}
	return
}

func AddToMap(ctx *runtime.Context, resultMap map[string]interface{}, obj interfaze.ConvertibleToMap) {
	key, value := obj.ToMap(ctx)
	if key != "" && value != nil {
		resultMap[key] = value
	}
}

func Yes(question string) bool {
	answer := Choice(question, 1, []string{"yes", "no"})
	return answer == "yes"
}

func No(question string) bool {
	answer := Choice(question, 2, []string{"yes", "no"})
	return answer == "yes"
}

func AskWithDefault(question, defaultValue string) string {
	for {
		var answer string
		fmt.Printf("%s\ndefault choice: [%s]\n\r", question, defaultValue)

		stdin.Scanln(&answer)

		if answer == "" {
			return defaultValue
		}
		return answer
	}
}

func Ask(question string) string {
	return Choice(question, 0, nil)
}

// defaultOption value starts from 1
func Choice(question string, defaultOption int, validValues []string) string {
	for {
		var answer string
		options := ""
		if validValues != nil {
			for i := range validValues {
				options = fmt.Sprintf("%s%s. %s\n", options, strconv.Itoa(i+1), validValues[i])
			}
			fmt.Printf("%s\n%sdefault choice: [%d]:\n\r", question, options, defaultOption)
		} else {
			fmt.Printf("%s\n\r", question)
		}

		stdin.Scanln(&answer)

		if validValues == nil {
			return answer
		}

		var option int
		var err error
		if answer == "" {
			option = defaultOption
		} else {
			option, err = strconv.Atoi(answer)
		}
		if err == nil && option > 0 && option <= len(validValues) {
			return validValues[option-1]
		}
		fmt.Printf("The value %s is not valid to this question.\nPlease retry to answer:\n", answer)
	}
}

// ChoiceIndex returns index of choice chosen
func ChoiceIndex(question string, defaultOption int, validValues []string) int {
	for {
		var answer string
		options := ""
		if validValues != nil {
			for i := range validValues {
				options = fmt.Sprintf("%s%s. %s\n", options, strconv.Itoa(i+1), validValues[i])
			}
			fmt.Printf("%s\n%sdefault choice: [%d]:\n\r", question, options, defaultOption)
		}
		stdin.Scanln(&answer)
		var option int
		var err error
		if answer == "" {
			option = defaultOption
		} else {
			option, err = strconv.Atoi(answer)
		}
		if err == nil && option > 0 && option <= len(validValues) {
			return option - 1
		}
		fmt.Printf("The value %s is not valid to this question.\nPlease retry to answer:\n", answer)
	}
}
func EnterToExit() {
	fmt.Println("Please press Enter to exit...")
	stdin.Scanln()
}
