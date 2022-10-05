package util

import (
	"log"
	"os"
	"strings"
)

func ReadFromFile(filename string) string {
	data, err := os.ReadFile(filename)
	if err != nil {
		PanicIfErr("", err)
	}
	str := string(data)
	return strings.ReplaceAll(str, "\r\n", "\n")
}

func PanicIfErr(message string, err error) {
	if err != nil {
		log.Panicf("%v %v", message, err)
	}
}
