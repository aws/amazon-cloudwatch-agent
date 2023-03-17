package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Rule func(map[string]interface{}) error

var rules []Rule

func AddRule(rule Rule) {
	rules = append(rules, rule)
}

func IsOldConfig(path string) (bool, error) {
	cf, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var conf map[string]interface{}

	if err := toml.Unmarshal(cf, &conf); err != nil {
		return false, fmt.Errorf("failed to unmarshal config file '%v': %v", path, err)
	}

	agent, ok := conf["agent"].(map[string]interface{})
	if !ok {
		return false, nil
	}

	if target, ok := agent["logtarget"].(string); ok && target == "lumberjack" {
		return false, nil
	}

	return true, nil
}

func MigrateFile(path string) (string, error) {
	cf, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var conf map[string]interface{}

	if err := toml.Unmarshal(cf, &conf); err != nil {
		return "", fmt.Errorf("failed to unmarshal config file '%v': %v", path, err)
	}

	dir, _ := filepath.Split(path)
	of, err := os.CreateTemp(dir, "migrated-*.conf")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary migrated config file: %v", err)
	}

	if err := Migrate(conf); err != nil {
		return "", err
	}

	if err := toml.NewEncoder(of).Encode(conf); err != nil {
		return "", err
	}

	if err := of.Close(); err != nil {
		return "", err
	}

	return of.Name(), nil
}

func Migrate(conf map[string]interface{}) error {
	for _, rule := range rules {
		err := rule(conf)
		if err != nil {
			return err
		}
	}
	return nil
}

func getItem(conf map[string]interface{}, section, plugin string) []map[string]interface{} {

	s, ok := conf[section].(map[string]interface{})
	if !ok {
		return nil
	}

	p, ok := s[plugin].([]map[string]interface{})
	if !ok {
		return nil
	}

	return p

}
