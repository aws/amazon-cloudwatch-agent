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
	var config IntegConfig
	err = json.Unmarshal(raw, &config)
	if err != nil {
		log.Fatal("Error during json.Unmarshall() in fetchIntegConfig(): ", err)
	}
	return config
}
