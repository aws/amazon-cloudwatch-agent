package integration

import (
	"fmt"
	"os"
	"path"
)

const varsFilename = "config_ignore.tfvars"

func WriteVarsFile(config Config) string {
	configVars := fetchVars(config)
	file, err := os.Create(varsFilename)
	LogFatalIfError(err)
	defer file.Close()
	for _, configVar := range configVars {
		_, err := file.WriteString(configVar + "\n")
		LogFatalIfError(err)
	}

	return getAbsoluteVarsFilepath()
}

func getAbsoluteVarsFilepath() string {
	wd, err := os.Getwd()
	LogFatalIfError(err)
	return path.Join(wd, varsFilename)
}

func fetchVars(config Config) []string {
	var vars []string
	for key, val := range config {
		key = mapMatrixRowFieldToTerraformVar(key)
		configVar := buildTfvar(key, val)
		vars = append(vars, configVar)
	}
	return vars
}

func mapMatrixRowFieldToTerraformVar(matrixRowField string) string {
	var exceptions = map[string]string{
		"test_dir":            "test_dir",
		"os":                  "test_name",
		"instanceType":        "ec2_instance_type",
		"installAgentCommand": "install_agent",
		"username":            "user",
	}

	if tfVarException, ok := exceptions[matrixRowField]; ok {
		return tfVarException
	}

	return ConvertCamelToSnakeCase(matrixRowField)
}

func buildTfvar(key string, val any) string {
	const (
		defaultFmt = "%v=%v"
		stringFmt  = `%v="%v"`
	)
	if _, ok := val.(string); ok {
		return fmt.Sprintf(stringFmt, key, val)
	} else {
		return fmt.Sprintf(defaultFmt, key, val)
	}
}
