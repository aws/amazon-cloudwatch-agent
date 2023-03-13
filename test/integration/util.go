package integration

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
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

func ExecCommandWithStderr(name, args string) error {
	cmd := exec.Command(name, strings.Split(args, " ")...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		m := scanner.Text()
		fmt.Println(m)
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func FetchSha() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	return string(out), err
}
