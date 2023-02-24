export const USE_CASE: string[] = ["statsd", "logs", "disk"];
export const REPORTED_METRICS: string[] = ["procstat_cpu_usage", "procstat_memory_rss"];
export const TRANSACTION_PER_MINUTE: number[] = [100, 1000, 5000];
export const OWNER_REPOSITORY: string = "aws";
export const SERVICE_NAME: string = "AmazonCloudWatchAgent";
export const CONVERT_REPORTED_METRICS_NAME: { [metric_name: string]: string } = {
  procstat_cpu_usage: "CPU Usage",
  procstat_memory_rss: "Memory Resource",
  procstat_memory_swap: "Memory Swap",
  procstat_memory_vms: "Virtual Memory",
  procstat_memory_data: "Swap Memory",
  procstat_num_fds: "File Descriptors",
  procstat_write_bytes: "Write Disk Bytes",
  net_bytes_sent: "Net Bytes Sent",
  net_packets_sent: "Net Packages Sent",
};