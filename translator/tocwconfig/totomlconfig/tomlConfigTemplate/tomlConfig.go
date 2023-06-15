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
		EcsDecorator []ecsDecoratorConfig
		EmfProcessor []emfProcessorConfig
		K8sDecorator []k8sDecoratorConfig
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
		Tags             map[string]string
	}

	eventConfig struct {
		BatchReadSize   int      `toml:"batch_read_size"`
		EventLevels     []string `toml:"event_levels"`
		EventName       string   `toml:"event_name"`
		LogGroupName    string   `toml:"log_group_name"`
		LogStreamName   string   `toml:"log_stream_name"`
		RetentionInDays int      `toml:"retention_in_days"`
	}

	logFileConfig struct {
		Destination     string
		FileStateFolder string       `toml:"file_state_folder"`
		FileConfig      []fileConfig `toml:"file_config"`
	}

	fileConfig struct {
		AutoRemoval     bool   `toml:"auto_removal"`
		FilePath        string `toml:"file_path"`
		FromBeginning   bool   `toml:"from_beginning"`
		LogGroupName    string `toml:"log_group_name"`
		LogStreamName   string `toml:"log_stream_name"`
		Pipe            bool
		RetentionInDays int `toml:"retention_in_days"`
		Timezone        string
		Tags            map[string]string
		Filters         []fileConfigFilter
	}

	k8sApiServerConfig struct {
		Interval string
		NodeName string `toml:"node_name"`
		Tags     map[string]string
	}

	memConfig struct {
		FieldPass []string
		Interval  string
		Tags      map[string]string
	}

	netConfig struct {
		FieldPass  []string
		Interfaces []string
		Tags       map[string]string
	}

	netStatConfig struct {
		FieldPass []string
		Interval  string
		Tags      map[string]string
	}

	nvidiaSmi struct {
		FieldPass  []string
		Interval   string
		TagExclude []string
		Tags       map[string]string
	}

	processesConfig struct {
		FieldPass []string
		Tags      map[string]string
	}

	prometheusConfig struct {
		ClusterName          string                              `toml:"cluster_name"`
		PrometheusConfigPath string                              `toml:"prometheus_config_path"`
		EcsServiceDiscovery  prometheusEcsServiceDiscoveryConfig `toml:"ecs_service_discovery"`
		Tags                 map[string]string
	}

	prometheusEcsServiceDiscoveryConfig struct {
		SdClusterRegion         string                    `toml:"sd_cluster_region"`
		SdFrequency             string                    `toml:"sd_frequency"`
		SdResultFile            string                    `toml:"sd_result_file"`
		SdTargetCluster         string                    `toml:"sd_target_cluster"`
		DockerLabel             map[string]string         `toml:"docker_label"`
		ServiceNameListForTasks []serviceNameListForTasks `toml:"service_name_list_for_tasks"`
		TaskDefinitionList      []taskDefinitionList      `toml:"task_definition_list"`
	}

	serviceNameListForTasks struct {
		SdContainerNamePattern string `toml:"sd_container_name_pattern"`
		SdJobName              string `toml:"sd_job_name"`
		SdMetricsPath          string `toml:"sd_metrics_path"`
		SdMetricsPorts         string `toml:"sd_metrics_ports"`
		SdServiceNamePattern   string `toml:"sd_service_name_pattern"`
	}

	taskDefinitionList struct {
		SdJobName                  string `toml:"sd_job_name"`
		SdMetricsPath              string `toml:"sd_metrics_path"`
		SdMetricsPorts             string `toml:"sd_metrics_ports"`
		SdTaskDefinitionArnPattern string `toml:"sd_task_definition_arn_pattern"`
	}

	procStatConfig struct {
		FieldPass  []string
		PidFile    string `toml:"pid_file"`
		PidFinder  string `toml:"pid_finder"`
		TagExclude []string
		Tags       map[string]string
	}

	socketListenerConfig struct {
		CollectdAuthFile      string   `toml:"collectd_auth_file"`
		CollectdSecurityLevel string   `toml:"collectd_security_level"`
		CollectdTypesDb       []string `toml:"collectd_typesdb"`
		DataFormat            string   `toml:"data_format"`
		NamePrefix            string   `toml:"name_prefix"`
		NameOverride          string   `toml:"name_override"`
		ServiceAddress        string   `toml:"service_address"`
		Tags                  map[string]string
	}

	statsdConfig struct {
		AllowedPendingMessages int `toml:"allowed_pending_messages"`
		Interval               string
		MetricSeparator        string `toml:"metric_separator"`
		ParseDataDogTags       bool   `toml:"parse_data_dog_tags"`
		ServiceAddress         string `toml:"service_address"`
		Tags                   map[string]string
	}

	swapConfig struct {
		FieldPass []string
		Tags      map[string]string
	}

	windowsEventLogConfig struct {
		Destination     string
		FileStateFolder string        `toml:"file_state_folder"`
		EventConfig     []eventConfig `toml:"event_config"`
		Tags            map[string]string
	}

	// Output plugins

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

	ecsDecoratorConfig struct {
		HostIp  string `toml:"host_ip"`
		Order   int
		TagPass map[string][]string
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

	k8sDecoratorConfig struct {
		ClusterName             string `toml:"cluster_name"`
		DisableMetricExtraction bool   `toml:"disable_metric_extraction"`
		HostIp                  string `toml:"host_ip"`
		NodeName                string `toml:"host_name_from_env"`
		Order                   int
		PreferFullPodName       bool `toml:"prefer_full_pod_name"`
		TagService              bool `toml:"tag_service"`
		TagPass                 map[string][]string
	}
)
