// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

type OldSsmCwConfig struct {
	IsEnabled           bool `json:"IsEnabled"`
	EngineConfiguration struct {
		PollInterval string `json:"PollInterval"`
		Components   []struct {
			ID         string `json:"Id"`
			FullName   string `json:"FullName"`
			Parameters struct {
				// Windows log events
				LogName string `json:"LogName"`
				Levels  string `json:"Levels"`
				// logs
				LogDirectoryPath string `json:"LogDirectoryPath"`
				TimestampFormat  string `json:"TimestampFormat"`
				Encoding         string `json:"Encoding"`
				Filter           string `json:"Filter"`
				CultureName      string `json:"CultureName"`
				TimeZoneKind     string `json:"TimeZoneKind"`
				LineCount        string `json:"LineCount"`
				// metrics
				CategoryName   string `json:"CategoryName"`
				CounterName    string `json:"CounterName"`
				InstanceName   string `json:"InstanceName"`
				MetricName     string `json:"MetricName"`
				Unit           string `json:"Unit"`
				DimensionName  string `json:"DimensionName"`
				DimensionValue string `json:"DimensionValue"`
				// output logs
				AccessKey string `json:"AccessKey"`
				SecretKey string `json:"SecretKey"`
				Region    string `json:"Region"`
				LogGroup  string `json:"LogGroup"`
				LogStream string `json:"LogStream"`
				// output metrics
				NameSpace string `json:"NameSpace"`
			} `json:"Parameters"`
		} `json:"Components"`
		Flows struct {
			Flows []string `json:"Flows"`
		} `json:"Flows"`
	} `json:"EngineConfiguration"`
}

type NewCwConfig struct {
	Agent   map[string]interface{} `json:"agent"`
	Metrics *MetricsEntry          `json:"metrics,omitempty"`
	Logs    *LogsEntry             `json:"logs,omitempty"`
}

type MetricsEntry struct {
	MetricsCollect   map[string]interface{} `json:"metrics_collected"`
	GlobalDimensions struct {
		ImageID              string `json:"ImageId"`
		InstanceID           string `json:"InstanceId"`
		InstanceType         string `json:"InstanceType"`
		AutoScalingGroupName string `json:"AutoScalingGroupName"`
	} `json:"append_dimensions"`
}

type LogsEntry struct {
	LogsCollected LogsCollectedEntry `json:"logs_collected,omitempty"`
}

type LogsCollectedEntry struct {
	Files         *FilesEntry         `json:"files,omitempty"`
	WindowsEvents *WindowsEventsEntry `json:"windows_events,omitempty"`
}

type FilesEntry struct {
	CollectList []NewCwConfigLog `json:"collect_list,omitempty"`
}

type WindowsEventsEntry struct {
	CollectList []NewCwConfigWindowsEventLog `json:"collect_list,omitempty"`
}

type NewCwConfigLog struct {
	FilePath                string `json:"file_path"`
	CloudwatchLogGroupName  string `json:"log_group_name"`
	CloudwatchLogStreamName string `json:"log_stream_name"`
	TimeZone                string `json:"timezone"`
}
type NewCwConfigWindowsEventLog struct {
	EventName               string   `json:"event_name"`
	EventLevels             []string `json:"event_levels"`
	CloudwatchLogGroupName  string   `json:"log_group_name"`
	CloudwatchLogStreamName string   `json:"log_stream_name"`
	EventFormat             string   `json:"event_format"`
}
type NewCwConfigMetric struct {
	Counters  []map[string]interface{}
	Instances []string
}
