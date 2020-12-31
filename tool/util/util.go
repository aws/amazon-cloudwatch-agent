// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	sysruntime "runtime"
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/tool/data/interfaze"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/stdin"

	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	configJsonFileName = "config.json"

	OsTypeLinux   = "linux"
	OsTypeWindows = "windows"
	OsTypeDarwin  = "darwin"

	MapKeyMetricsCollectionInterval = "metrics_collection_interval"
	MapKeyInstances                 = "resources"
	MapKeyMeasurement               = "measurement"
)

func CurOS() string {
	return sysruntime.GOOS
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
	err := ioutil.WriteFile(filePath, []byte(""), 0755)
	if err != nil {
		fmt.Printf("Make sure that you have write permission to %s\n", filePath)
		os.Exit(1)
	}
}

func ReadConfigFromJsonFile() string {
	filePath := ConfigFilePath()
	byteArray, err := ioutil.ReadFile(filePath)
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

func SaveResultByteArrayToJsonFile(resultByteArray []byte) string {
	filePath := ConfigFilePath()
	err := ioutil.WriteFile(filePath, resultByteArray, 0755)
	if err != nil {
		fmt.Printf("Error in writing file to %s: %v\nMake sure that you have write permission to %s.", filePath, err, filePath)
		os.Exit(1)
	}
	fmt.Printf("Saved config file to %s successfully.\n", filePath)
	return filePath
}

func SDKRegion() (region string) {
	ses, e := session.NewSession()

	if e != nil {
		return
	}
	if ses.Config != nil && ses.Config.Region != nil {
		region = *ses.Config.Region
	}
	return region
}

func SDKRegionWithProfile(profile string) (region string) {
	ses, e := session.NewSessionWithOptions(session.Options{Profile: profile, SharedConfigState: session.SharedConfigEnable})

	if e != nil {
		return
	}
	if ses.Config != nil && ses.Config.Region != nil {
		region = *ses.Config.Region
	}
	return region
}

func SDKCredentials() (accessKey, secretKey string, creds *credentials.Credentials) {
	ses, e := session.NewSession()
	if e != nil {
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
	ses, e := session.NewSession(&aws.Config{
		HTTPClient: &http.Client{Timeout: 1 * time.Second},
		MaxRetries: aws.Int(0),
	})
	if e != nil {
		return
	}
	md := ec2metadata.New(ses)
	if !md.Available() {
		return
	}
	if info, e := md.Region(); e == nil {
		region = info
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
	if answer == "yes" {
		return true
	}
	return false
}

func No(question string) bool {
	answer := Choice(question, 2, []string{"yes", "no"})
	if answer == "yes" {
		return true
	}
	return false
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

func EnterToExit() {
	fmt.Println("Please press Enter to exit...")
	stdin.Scanln()
}
