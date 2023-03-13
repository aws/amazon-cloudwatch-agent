package integration

import (
	"fmt"
	"os"
	"path"
)

const varsFilename = "config_ignore.tfvars"

func WriteVarsFile(integConfig IntegConfig) string {
	integConfigVars := fetchVars(integConfig)
	file, err := os.Create(varsFilename)
	LogFatalIfError(err)
	defer file.Close()
	for _, integConfigVar := range integConfigVars {
		_, err := file.WriteString(integConfigVar + "\n")
		LogFatalIfError(err)
	}

	return getAbsoluteVarsFilepath()
}

func getAbsoluteVarsFilepath() string {
	wd, err := os.Getwd()
	LogFatalIfError(err)
	return path.Join(wd, varsFilename)
}

func fetchVars(integConfig IntegConfig) []string {
	var vars []string
	for key, val := range integConfig {
		key = mapMatrixRowFieldToTerraformVar(key)
		integConfigVar := buildTfvar(key, val)
		vars = append(vars, integConfigVar)
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
