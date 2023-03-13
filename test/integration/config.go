package integration

import (
	"encoding/json"
	"log"
	"os"
)

type IntegConfig map[string]any

func FetchIntegConfig() IntegConfig {
	const configPath = "config_ignore.json"
	raw, err := os.ReadFile(configPath)
	LogFatalIfError(err)
	var integConfig IntegConfig
	err = json.Unmarshal(raw, &integConfig)
	if err != nil {
		log.Fatal("Error during json.Unmarshall() in fetchIntegConfig(): ", err)
	}
	fillCwaSha(integConfig)
	return integConfig
}

func fillCwaSha(integConfig IntegConfig) {
	_, ok := integConfig["cwaGithubSha"]
	if !ok {
		currentSha, err := GetSha()
		if err != nil {
			log.Fatalf("Error GetSha(): %v", err)
		}
		integConfig["cwaGithubSha"] = currentSha
	}

}
