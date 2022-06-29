package util

import (
	"io/ioutil"
	"log"
	"strings"
)

func ReadFromFile(filename string) string {
	data, err := ioutil.ReadFile(filename)
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
