package integration

import (
	"encoding/json"
	"log"
	"os"
)

type IntegConfig map[string]any

func FetchIntegConfig() IntegConfig {
	const configPath = "config.json"
	raw, err := os.ReadFile(configPath)
	LogFatalIfError(err)
	var integConfig IntegConfig
	err = json.Unmarshal(raw, &integConfig)
	if err != nil {
		log.Fatal("Error during json.Unmarshall() in fetchIntegConfig(): ", err)
	}
	fillInDefaultValues(integConfig)
	PrettyPrint(integConfig)
	return integConfig
}

var defaultValues = map[string]string{
	"githubTestRepo":       "https://github.com/aws/amazon-cloudwatch-agent-test.git",
	"githubTestRepoBranch": "main",
	"pluginTests":          "",
}

func fillInDefaultValues(integConfig IntegConfig) {
	fillShaIfEmpty(integConfig)
	for key, val := range defaultValues {
		fillIfEmpty(integConfig, key, val)
	}
}

func fillIfEmpty(integConfig IntegConfig, key, val string) {
	_, ok := integConfig[key]
	if !ok {
		integConfig[key] = val

	}
}

func fillShaIfEmpty(integConfig IntegConfig) {
	currentSha, err := FetchSha()
	if err != nil {
		log.Fatalf("Error GetSha(): %v", err)
	}
	fillIfEmpty(integConfig, "cwaGithubSha", currentSha)
}
