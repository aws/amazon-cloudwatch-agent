package util

import (
	"fmt"
	"os"
)

// environment var definition
const (
	SystemDrive = "SystemDrive"
	ProgramData = "ProgramData"
)

func GetWindowsSystemDrivePath() string {
	return getEnvWithDefaultVal(SystemDrive, "C:")
}

func GetWindowsProgramDataPath() string {
	return getEnvWithDefaultVal(ProgramData, GetWindowsSystemDrivePath()+"\\ProgramData")
}

func getEnvWithDefaultVal(envName string, defaultVal string) string {
	envVal := os.Getenv(envName)
	if envVal == "" {
		fmt.Printf("can't get environment var: %v, use default value: %v \n", envName, defaultVal)
		envVal = defaultVal
	}
	return envVal
}
