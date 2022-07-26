// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"os/exec"
	"strings"
	"strconv"
)

const (
	linux   = "linux"
	windows = "windows"
	mac     = "mac"
	testDir = "test_dir"
)

//you can't have a const map in golang
var osToTestDirMap = map[string][]string{
	"ec2_linux": {
		"./integration/test/ca_bundle",
		"./integration/test/cloudwatchlogs",
		"./integration/test/metrics_number_dimension",
	},
	// @TODO add real tests
	"ec2_performance":{
		"./integration/test/performancetest",
	},
	"ec2_windows": {""},
	"ec2_mac":     {},
	"ecs_fargate": {
		"./integration/test/ecs/ecs_metadata",
	},
}

func main() {
	releases := getReleases(1627262978,1658798978)
	if len(releases) ==0{
		log.Panicf("CRASH")
	}
	// for osType, testDir := range osToTestDirMap {
	// 	testMatrix := genMatrix(osType, testDir)
	// 	writeTestMatrixFile(osType, testMatrix)
	// }
}

func genMatrix(targetOS string, testDirList []string) []map[string]string {
	openTestMatrix, err := os.Open(fmt.Sprintf("integration/generator/resources/%v_test_matrix.json", targetOS))
	
	if err != nil {
		log.Panicf("can't read file %v_test_matrix.json err %v", targetOS, err)
	}
	
	byteValueTestMatrix, _ := ioutil.ReadAll(openTestMatrix)
	_ = openTestMatrix.Close()
	
	var testMatrix []map[string]string
	err = json.Unmarshal(byteValueTestMatrix, &testMatrix)
	if err != nil {
		log.Panicf("can't unmarshall file %v_test_matrix.json err %v", targetOS, err)
	}

	var testMatrixComplete []map[string]string
	for _, test := range testMatrix {
		for _, testDirectory := range testDirList {
			testLine := copyMap(test)
			testLine[testDir] = testDirectory
			testMatrixComplete = append(testMatrixComplete, testLine)
		}
	}
	return testMatrixComplete
}

func writeTestMatrixFile(targetOS string, testMatrix []map[string]string) {
	bytes, err := json.MarshalIndent(testMatrix, "", " ")
	if err != nil {
		log.Panicf("Can't marshal json for target os %v, err %v", targetOS, err)
	}
	err = ioutil.WriteFile(fmt.Sprintf("integration/generator/resources/%v_complete_test_matrix.json", targetOS), bytes, os.ModePerm)
	if err != nil {
		log.Panicf("Can't write json to file for target os %v, err %v", targetOS, err)
	}
}

func copyMap(mapToCopy map[string]string) map[string]string {
	testLine := make(map[string]string)
	for key, value := range mapToCopy {
		testLine[key] = value
	}
	return testLine
}

func genMatrixForReleases(targetOS string, testDirList []string) []map[string]string {
	openTestMatrix, err := os.Open(fmt.Sprintf("integration/generator/resources/%v_test_matrix.json", targetOS))
	
	if err != nil {
		log.Panicf("can't read file %v_test_matrix.json err %v", targetOS, err)
	}
	
	byteValueTestMatrix, _ := ioutil.ReadAll(openTestMatrix)
	_ = openTestMatrix.Close()
	
	var testMatrix []map[string]string
	err = json.Unmarshal(byteValueTestMatrix, &testMatrix)
	if err != nil {
		log.Panicf("can't unmarshall file %v_test_matrix.json err %v", targetOS, err)
	}

	// fmt.Println(testMatrix)
	releases := getReleases(1627262978,1658798978)
	var testMatrixComplete []map[string]string
	for  _, release := range releases{
		for i, _ := range testMatrix {
			testMatrix[i]["commitSHA"] = release
			// fmt.Println(test)
			for _, testDirectory := range testDirList {
				testLine := copyMap(testMatrix[i])
				testLine[testDir] = testDirectory
				testMatrixComplete = append(testMatrixComplete, testLine)
			}
		}
	}
	bytes, err := json.MarshalIndent(testMatrixComplete, "", " ")
	if err != nil {
		log.Panicf("Can't marshal json for target os %v, err %v", targetOS, err)
	}
	err = ioutil.WriteFile(fmt.Sprintf("integration/generator/resources/%v_test_matrix.json", targetOS), bytes, os.ModePerm)
	return testMatrixComplete
}

func getReleases(startDate int ,EndDate int ) []string{
	cmd := exec.Command("git", "show" ,"--tags" )
	rawTags, _:= cmd.Output()
	tagData := strings.Split(string(rawTags),"\n")
	var tagList []string
	fmt.Println(tagData)
	i :=0
	for _,element := range tagData{
		data := strings.Split(element,"|")
		date,_ := strconv.Atoi(data[0])
		// fmt.Println(date)
		if i > 2{
			break
		}
		if date > startDate && date < EndDate{
			tagList = append(tagList,data[1])
			i++
		}
	}
	fmt.Println(tagList)
	return tagList
}