package integration

import (
	"log"
	"os"
	"path"
	"strings"
	"unicode"
)

func LogFatalIfError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// ConvertCamelToSnakeCase converts a string from camel to snake case
// e.g. helloThere => hello_there
func ConvertCamelToSnakeCase(camel string) string {
	var words []string
	lo := 0
	for hi, char := range camel {
		if unicode.IsUpper(char) {
			word := strings.ToLower(camel[lo:hi])
			if len(word) > 0 {
				words = append(words, word)
			}
			lo = hi
		}
	}
	words = append(words, strings.ToLower(camel[lo:]))
	return strings.Join(words, "_")
}

func GetRootDir() string {
	wd, _ := os.Getwd()
	rootDir := path.Join(wd, "../")
	return rootDir
}
