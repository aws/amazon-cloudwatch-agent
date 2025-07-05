// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tomlConfigTemplate

type (
	TomlConfig struct {
		Agent      agentConfig
		Inputs     inputConfig
		Outputs    outputConfig
		Processors processorsConfig
	}

	agentConfig struct {
		// Not all names need the explicit toml mapping as they are case insensitive, it is only needed when
		// underscore is replaced
		CollectionJitter  string `toml:"collection_jitter"`
		Debug             bool
		FlushInterval     string `toml:"flush_interval"`
		FlushJitter       string `toml:"flush_jitter"`
		Hostname          string
		Interval          string
		Logfile           string
		LogTarget         string
		MetricBatchSize   int  `toml:"metric_batch_size"`
		MetricBufferLimit int  `toml:"metric_buffer_limit"`
		OmitHostname      bool `toml:"omit_hostname"`
		Precision         string
		Quiet             bool
		RoundInterval     bool `toml:"round_interval"`
	}

	inputConfig struct {
		Cadvisor        []cadvisorConfig
		Cpu             []cpuConfig
		Disk            []diskConfig
		DiskIo          []diskioConfig
		Ethtool         []ethtoolConfig
		K8sapiserver    []k8sApiServerConfig
		Logfile         []logFileConfig
		Mem             []memConfig
		Net             []netConfig
		NetStat         []netStatConfig
		NvidiaSmi       []nvidiaSmi `toml:"nvidia_smi"`
		Processes       []processesConfig
		Prometheus      []prometheusConfig `toml:"prometheus"`
		ProcStat        []procStatConfig
		SocketListener  []socketListenerConfig `toml:"socket_listener"`
		Statsd          []statsdConfig
		Swap            []swapConfig
		WindowsEventLog []windowsEventLogConfig `toml:"windows_event_log"`
	}

	outputConfig struct {
		CloudWatch     []cloudWatchOutputConfig
		CloudWatchLogs []cloudWatchLogsConfig
	}

	processorsConfig struct {
		Delta        []processorDelta
		EmfProcessor []emfProcessorConfig
	}

	// Input Plugins

	cadvisorConfig struct {
		ContainerOrchestrator string `toml:"container_orchestrator"`
		Interval              string
		Mode                  string
		Tags                  map[string]string
	}

	cpuConfig struct {
		CollectCpuTime bool `toml:"collect_cpu_time"`
		FieldPass      []string
		Interval       string
		PerCpu         bool
		ReportActive   bool `toml:"report_active"`
		TotalCpu       bool
		Tags           map[string]string
	}

	diskConfig struct {
		FieldPass   []string
		IgnoreFs    []string `toml:"ignore_fs"`
		Interval    string
		MountPoints []string `toml:"mount_points"`
		TagExclude  []string
		Tags        map[string]string
	}

	diskioConfig struct {
		FieldPass []string
		Interval  string
	}

	ethtoolConfig struct {
		FieldPass        []string
		InterfaceInclude []string `toml:"interface_include"`
		Interval         string
	}

	k8sApiServerConfig struct {
		Interval string
		Tags     map[string]string
	}

	logFileConfig struct {
		FileConfig       []fileConfig `toml:"file_config"`
		ForceFlushInterv string       `toml:"force_flush_interval"`
	}

	fileConfig struct {
		FileDirectory    string `toml:"file_directory"`
		FileNameFilter   string `toml:"file_name_filter"`
		FileStateFolder  string `toml:"file_state_folder"`
		FromBeginning    bool   `toml:"from_beginning"`
		LogGroupName     string `toml:"log_group_name"`
		LogStreamName    string `toml:"log_stream_name"`
		MultiLinePattern string `toml:"multi_line_pattern"`
		Timezone         string
		Filters          []fileConfigFilter
		RetentionInDays  int `toml:"retention_in_days"`
		AutoRemoval      bool
		TagsKey          string `toml:"tags_key"`
		Tags             map[string]string
	}

	memConfig struct {
		FieldPass []string
		Interval  string
		Tags      map[string]string
	}

	netConfig struct {
		FieldPass []string
		Interval  string
		Tags      map[string]string
	}

	netStatConfig struct {
		FieldPass []string
		Interval  string
		Tags      map[string]string
	}

	nvidiaSmi struct {
		Timeout  int
		Interval string
		Tags     map[string]string
	}

	processesConfig struct {
		FieldPass []string
		Interval  string
		Tags      map[string]string
	}

	prometheusConfig struct {
		ClusterName                string   `toml:"cluster_name"`
		EndpointURLs               []string `toml:"endpoint_urls"`
		GlobalMetricsDeclarations  []metricsDeclaration
		GlobalDimensionsBlacklist  []string `toml:"global_dimensions_blacklist"`
		GlobalTagsInclude          []string `toml:"global_tags_include"`
		Interval                   string
		MetricsDeclarations        []metricsDeclaration `toml:"metrics_declaration"`
		OutputDestination          string               `toml:"output_destination"`
		ServiceAddressURLLabelName string               `toml:"service_address_url_label_name"`
		SourceLabelsBlacklist      []string             `toml:"source_labels_blacklist"`
		TagsKey                    string               `toml:"tags_key"`
		Tags                       map[string]string
	}

	metricsDeclaration struct {
		DimensionNameRequirement []string `toml:"dimension_name_requirement"`
		LabelMatchers            []string `toml:"label_matchers"`
		MetricNameSelectors      []string `toml:"metric_name_selectors"`
		MetricRegistryName       string   `toml:"metric_registry_name"`
		SourceLabels             []string `toml:"source_labels"`
		TargetMetricNames        []string `toml:"target_metric_names"`
	}

	procStatConfig struct {
		FieldPass []string
		Interval  string
		Pattern   string
		PidFile   string `toml:"pid_file"`
		PidFinder string `toml:"pid_finder"`
		PidTag    bool   `toml:"pid_tag"`
		Tags      map[string]string
	}

	socketListenerConfig struct {
		ServiceAddress  string `toml:"service_address"`
		DataFormat      string `toml:"data_format"`
		ContentEncoding string `toml:"content_encoding"`
		KeepAlivePeriod string `toml:"keep_alive_period"`
		MaxConnections  int    `toml:"max_connections"`
		ReadBufferSize  int    `toml:"read_buffer_size"`
		ReadTimeout     string `toml:"read_timeout"`
		MetricSeparator string `toml:"metric_separator"`
		TagsKey         string `toml:"tags_key"`
		Tags            map[string]string
	}

	statsdConfig struct {
		AllowedPendingMessages int      `toml:"allowed_pending_messages"`
		ConvertNames           bool     `toml:"convert_names"`
		DeleteCounters         bool     `toml:"delete_counters"`
		DeleteGauges           bool     `toml:"delete_gauges"`
		DeleteSets             bool     `toml:"delete_sets"`
		DeleteTimings          bool     `toml:"delete_timings"`
		MetricSeparator        string   `toml:"metric_separator"`
		ParseDataDogTags       bool     `toml:"parse_data_dog_tags"`
		ServiceAddress         string   `toml:"service_address"`
		PercentileLimit        int      `toml:"percentile_limit"`
		Percentiles            []int    `toml:"percentiles"`
		Templates              []string `toml:"templates"`
		TagsKey                string   `toml:"tags_key"`
		Tags                   map[string]string
	}

	swapConfig struct {
		FieldPass []string
		Interval  string
		Tags      map[string]string
	}

	windowsEventLogConfig struct {
		EventName     string `toml:"event_name"`
		EventLevels   []string
		BatchReadSize int    `toml:"batch_read_size"`
		StartAt       string `toml:"start_at"`
		Destination   string
		Tags          map[string]string
	}

	// Output Plugins

	cloudWatchOutputConfig struct {
		EndpointOverride    string `toml:"endpoint_override"`
		ForceFlushInterval  string `toml:"force_flush_interval"`
		MaxDatumsPerCall    int    `toml:"max_datums_per_call"`
		MaxValuesPerDatum   int    `toml:"max_values_per_datum"`
		Namespace           string
		Region              string
		RoleArn             string     `toml:"role_arn"`
		RollupDimensions    [][]string `toml:"rollup_dimensions"`
		TagExclude          []string
		DropOriginalMetrics map[string][]string      `toml:"drop_original_metrics"`
		MetricDecorations   []metricDecorationConfig `toml:"metric_decoration"`
		TagPass             map[string][]string
	}

	metricDecorationConfig struct {
		Category string
		Name     string
		Rename   string
		Unit     string
	}

	cloudWatchLogsConfig struct {
		Concurrency        int    `toml:"concurrency"`
		EndpointOverride   string `toml:"endpoint_override"`
		ForceFlushInterval string `toml:"force_flush_interval"`
		LogStreamName      string `toml:"log_stream_name"`
		Region             string
		RoleArn            string `toml:"role_arn"`
		TagExclude         []string
		TagPass            map[string][]string
	}

	fileConfigFilter struct {
		Expression string
		Type       string
	}

	// Processors
	processorDelta struct {
	}

	emfProcessorConfig struct {
		MetricDeclarationDedup bool   `toml:"metric_declaration_dedup"`
		MetricNamespace        string `toml:"metric_namespace"`
		Order                  int
		MetricDeclaration      []emfProcessorMetricDeclaration `toml:"metric_declaration"`
		MetricUnit             map[string]string               `toml:"metric_unit"`
		TagPass                map[string][]string
	}

	emfProcessorMetricDeclaration struct {
		Dimensions     [][]string
		LabelMatcher   string   `toml:"label_matcher"`
		LabelSeparator string   `toml:"label_separator"`
		MetricSelector []string `toml:"metric_selectors"`
		SourceLabels   []string `toml:"source_labels"`
	}
)
