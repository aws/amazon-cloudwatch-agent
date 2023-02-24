export const SUPPORTED_PLUGINs = [
    { input: "cpu", processor: "ec2tagger", output: "cloudwatch" },
    { input: "disk", processor: "delta", output: "cloudwatchlogs" },
    { input: "diskio", processor: "ecsdecorator", output: "" },
    { input: "ethtool", processor: "emfProcessor", output: "" },
    { input: "mem", processor: "k8sdecorator", output: "" },
    { input: "net", processor: "", output: "" },
    { input: "nvidia_smi", processor: "", output: "" },
    { input: "processes", processor: "", output: "" },
    { input: "procstat", processor: "", output: "" },
    { input: "collectd", processor: "", output: "" },
    { input: "emf", processor: "", output: "" },
    { input: "prometheus", processor: "", output: "" },
    { input: "awscsm", processor: "", output: "" },
    { input: "cadvisor", processor: "", output: "" },
    { input: "k8sapiserver", processor: "", output: "" },
    { input: "logfile", processor: "", output: "" },
    { input: "windows_event_log", processor: "", output: "" },
    { input: "win_perf_counters", processor: "", output: "" },
  ];
  
  export const SUPPORTED_USE_CASES = [
    {
      name: "ECS Container Insight",
      url: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/deploy-container-insights-ECS.html",
    },
    {
      name: "EKS Container Insight",
      url: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/deploy-container-insights-EKS.html",
    },
    {
      name: "Prometheus",
      url: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-install-EKS.html",
    },
    {
      name: "EMF",
      url: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Generation_CloudWatch_Agent.html",
    },
    {
      name: "Collectd",
      url: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-custom-metrics-collectd.html",
    },
    {
      name: "Statsd",
      url: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-custom-metrics-statsd.html",
    },
  ];