// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Jeffail/gabs"
)

const (
	ERROR       = "ERROR"
	WARNING     = "WARNING"
	INFORMATION = "INFORMATION"
)

func MapOldWindowsConfigToNewConfig(oldConfig OldSsmCwConfig) (newConfig NewCwConfig) {
	// Add static part
	newConfig.Logs = nil
	newConfig.Metrics = nil
	// Prepare for metrics addition
	foundMetrics := make(map[string]NewCwConfigMetric)

	// Prepare for the agent part addition
	newConfig.Agent = make(map[string]interface{})
	jsonObjAgent, _ := gabs.Consume(newConfig.Agent)

	// Get the log group names
	replacer := strings.NewReplacer("(", "", ")", "")
	foundLogGroupNames := make(map[string]string)
	foundLogStreamNames := make(map[string]string)
	for _, component := range oldConfig.EngineConfiguration.Components {
		if component.FullName == "AWS.EC2.Windows.CloudWatch.CloudWatchLogsOutput,AWS.EC2.Windows.CloudWatch" {
			for _, flow := range oldConfig.EngineConfiguration.Flows.Flows {
				if strings.Contains(flow, component.ID) {
					flowIds := strings.Split(replacer.Replace(flow), ",")
					for _, mId := range flowIds {
						foundLogGroupNames[mId] = component.Parameters.LogGroup
						foundLogStreamNames[mId] = component.Parameters.LogStream
					}
				}
			}
		}

		// This will panic if different regions/credentials exist because the new agent does not support multiple values
		// TODO: Capture different regions when the new agent is capable of https://github.com/aws/amazon-cloudwatch-agent/issues/230
		if component.Parameters.Region != "" {
			if val, ok := newConfig.Agent["region"]; ok && val != component.Parameters.Region {
				fmt.Fprint(os.Stderr, "Detected multiple different regions in the input config file. This feature is unsupported by the new agent. Thus, will not be able to migrate the old config. Terminating.")
				os.Exit(1)
			}
			jsonObjAgent.Set(component.Parameters.Region, "region")
		}
		if component.Parameters.AccessKey != "" {
			if creds, ok := newConfig.Agent["credentials"]; ok {
				if credsMap, valid := creds.(map[string]interface{}); valid {
					if val, ok := credsMap["access_key"]; ok && val != component.Parameters.AccessKey {
						fmt.Fprint(os.Stderr, "Detected multiple different access keys in the input config file. This feature is unsupported by the new agent. Thus, will not be able to migrate the old config. Terminating.")
						os.Exit(1)
					}
				}
			}
			jsonObjAgent.SetP(component.Parameters.AccessKey, "credentials.access_key")
		}
		if component.Parameters.SecretKey != "" {
			if creds, ok := newConfig.Agent["credentials"]; ok {
				if credsMap, valid := creds.(map[string]interface{}); valid {
					if val, ok := credsMap["secret_key"]; ok && val != component.Parameters.SecretKey {
						fmt.Fprint(os.Stderr, "Detected multiple different secret keys in the input config file. This feature is unsupported by the new agent. Thus, will not be able to migrate the old config. Terminating.")
						os.Exit(1)
					}
				}
			}
			jsonObjAgent.SetP(component.Parameters.SecretKey, "credentials.secret_key")
		}
	}

	// Get the repeated components (event logs, logs, and metrics)
	for _, component := range oldConfig.EngineConfiguration.Components {
		switch component.FullName {
		case "AWS.EC2.Windows.CloudWatch.CustomLog.CustomLogInputComponent,AWS.EC2.Windows.CloudWatch":
			if foundLogGroupNames[component.ID] != "" && foundLogStreamNames[component.ID] != "" {
				if newConfig.Logs == nil {
					newConfig.Logs = &LogsEntry{}
				}
				mLog := NewCwConfigLog{
					FilePath:                component.Parameters.LogDirectoryPath,
					CloudwatchLogGroupName:  foundLogGroupNames[component.ID],
					TimeZone:                component.Parameters.TimeZoneKind,
					CloudwatchLogStreamName: foundLogStreamNames[component.ID],
				}
				if !strings.HasSuffix(mLog.FilePath, "\\") {
					mLog.FilePath = mLog.FilePath + "\\"
				}
				if component.Parameters.Filter == "" {
					mLog.FilePath = mLog.FilePath + "*"
				} else {
					mLog.FilePath = mLog.FilePath + component.Parameters.Filter
				}
				if newConfig.Logs.LogsCollected.Files == nil {
					newConfig.Logs.LogsCollected.Files = &FilesEntry{}
				}
				newConfig.Logs.LogsCollected.Files.CollectList = append(newConfig.Logs.LogsCollected.Files.CollectList, mLog)
			}
		case "AWS.EC2.Windows.CloudWatch.EventLog.EventLogInputComponent,AWS.EC2.Windows.CloudWatch":
			if foundLogGroupNames[component.ID] != "" && foundLogStreamNames[component.ID] != "" {
				if newConfig.Logs == nil {
					newConfig.Logs = &LogsEntry{}
				}
				mLog := NewCwConfigWindowsEventLog{
					EventName:               component.Parameters.LogName,
					EventLevels:             mapLogLevelsStringToSlice(component.Parameters.Levels),
					CloudwatchLogGroupName:  foundLogGroupNames[component.ID],
					CloudwatchLogStreamName: foundLogStreamNames[component.ID],
					EventFormat:             "text",
				}
				if len(mLog.EventLevels) > 0 {
					if newConfig.Logs.LogsCollected.WindowsEvents == nil {
						newConfig.Logs.LogsCollected.WindowsEvents = &WindowsEventsEntry{}
					}
					newConfig.Logs.LogsCollected.WindowsEvents.CollectList = append(newConfig.Logs.LogsCollected.WindowsEvents.CollectList, mLog)
				}
			}
		case "AWS.EC2.Windows.CloudWatch.PerformanceCounterComponent.PerformanceCounterInputComponent,AWS.EC2.Windows.CloudWatch":
			if mMetric, ok := foundMetrics[component.Parameters.CategoryName]; ok {
				newMeasurement := make(map[string]interface{})
				jsonObjNewMeasurement, _ := gabs.Consume(newMeasurement)
				jsonObjNewMeasurement.Set(component.Parameters.CounterName, "name")
				if component.Parameters.MetricName != "" {
					jsonObjNewMeasurement.Set(component.Parameters.MetricName, "rename")
				}
				if component.Parameters.Unit != "" {
					jsonObjNewMeasurement.Set(component.Parameters.Unit, "unit")
				}
				mMetric.Counters = append(mMetric.Counters, newMeasurement)

				if component.Parameters.InstanceName != "" {
					mMetric.Instances = append(mMetric.Instances, component.Parameters.InstanceName)
				}
				foundMetrics[component.Parameters.CategoryName] = mMetric
			} else {
				newMetric := NewCwConfigMetric{
					Counters: []map[string]interface{}{{}},
				}
				jsonObjMeasurement, _ := gabs.Consume(newMetric.Counters[0])
				jsonObjMeasurement.Set(component.Parameters.CounterName, "name")
				if component.Parameters.MetricName != "" {
					jsonObjMeasurement.Set(component.Parameters.MetricName, "rename")
				}
				if component.Parameters.Unit != "" {
					jsonObjMeasurement.Set(component.Parameters.Unit, "unit")
				}

				if component.Parameters.InstanceName != "" {
					newMetric.Instances = []string{component.Parameters.InstanceName}
				}
				foundMetrics[component.Parameters.CategoryName] = newMetric
			}
		}
	}

	if len(foundMetrics) == 0 {
		return
	}

	// Add the metrics correctly
	newConfig.Metrics = &MetricsEntry{}
	newConfig.Metrics.GlobalDimensions.AutoScalingGroupName = "${aws:AutoScalingGroupName}"
	newConfig.Metrics.GlobalDimensions.ImageID = "${aws:ImageId}"
	newConfig.Metrics.GlobalDimensions.InstanceID = "${aws:InstanceId}"
	newConfig.Metrics.GlobalDimensions.InstanceType = "${aws:InstanceType}"
	newConfig.Metrics.MetricsCollect = make(map[string]interface{})
	jsonObj, _ := gabs.Consume(newConfig.Metrics.MetricsCollect)
	for key, mMetric := range foundMetrics {
		jsonObj.Set(mMetric.Counters, key, "measurement")
		if len(mMetric.Instances) > 0 {
			jsonObj.Set(mMetric.Instances, key, "resources")
		} else {
			jsonObj.Set([]string{}, key, "resources")
		}
	}

	return
}

func mapLogLevelsStringToSlice(levels string) []string {
	switch levels {
	case "1":
		return []string{ERROR}
	case "2":
		return []string{WARNING}
	case "3":
		return []string{ERROR, WARNING}
	case "4":
		return []string{INFORMATION}
	case "5":
		return []string{ERROR, INFORMATION}
	case "6":
		return []string{WARNING, INFORMATION}
	case "7":
		return []string{ERROR, WARNING, INFORMATION}
	default:
		log.Printf("Incorrect Levels token of value %s. The corresponding windows event log will be ignored.", levels)
		return []string{}
	}
}
