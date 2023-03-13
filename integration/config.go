package integration

import (
	"encoding/json"
	"log"
	"os"
)

const ConfigTfvarsFilename = "config_ignore.tfvars"

type Config map[string]any

func FetchConfig() Config {
	const configPath = "config.json"
	raw, err := os.ReadFile(configPath)
	LogFatalIfError(err)
	var config Config
	err = json.Unmarshal(raw, &config)
	if err != nil {
		log.Fatal("Error during json.Unmarshall() in fetchConfig(): ", err)
	}
	return config
}
