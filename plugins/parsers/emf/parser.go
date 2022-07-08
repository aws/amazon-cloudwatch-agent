// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
)

type EMFParser struct {
	MetricName  string
	DefaultTags map[string]string
}

type awsMetadata struct {
	LogGroupName  string `json:LogGroupName`
	LogStreamName string `json:LogStreamName`
}

type emfMetadata struct {
	AWSMetadata   *awsMetadata `json:"_aws"`
	LogGroupName  string       `json:"log_group_name"`
	LogStreamName string       `json:"log_stream_name"`
}

// One byte array can possibly have multiple structured log entries.
// The separator between multiple structured log entries is newline '\n'.
// It means that json will be sent in minimized/compact mode without any newline in the json body.
func (v *EMFParser) Parse(buf []byte) ([]telegraf.Metric, error) {
	vString := string(bytes.TrimSpace(bytes.Trim(buf, "\x00")))
	lines := strings.Split(vString, "\n")
	metrics := []telegraf.Metric{}
	for _, line := range lines {
		if metric, err := v.ParseLine(line); err == nil {
			metrics = append(metrics, metric)
		} else {
			log.Printf("W! Cannot parse line %s : %v", line, err)
		}
	}
	return metrics, nil
}

// One and only one metric is expected.
// If it is empty string or somehow cannot form one metric, it will return error.
func (v *EMFParser) ParseLine(line string) (telegraf.Metric, error) {
	line = strings.TrimSpace(line)
	metadata := new(emfMetadata)
	err := json.Unmarshal([]byte(line), metadata)
	if err != nil {
		return nil, fmt.Errorf("cannot serialize %s to json: %v", line, err)
	}
	var logGroupName, logStreamName string
	if metadata.AWSMetadata != nil {
		// v1
		logGroupName = metadata.AWSMetadata.LogGroupName
		logStreamName = metadata.AWSMetadata.LogStreamName
	} else {
		// v0
		logGroupName = metadata.LogGroupName
		logStreamName = metadata.LogStreamName
	}
	if logGroupName == "" {
		return nil, fmt.Errorf("log group name is required to send as structured log: %s", line)
	}

	fields := map[string]interface{}{"value": line}
	metric := metric.New(v.MetricName, v.DefaultTags, fields, time.Now().UTC())
	metric.AddTag(logscommon.LogGroupNameTag, logGroupName)
	// if the log stream name is empty, it will use the default log stream name set by output plugin
	if logStreamName != "" {
		metric.AddTag(logscommon.LogStreamNameTag, logStreamName)
	}

	return metric, nil
}

func (v *EMFParser) SetDefaultTags(tags map[string]string) {
	v.DefaultTags = tags
}
