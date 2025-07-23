export interface CloudWatchConfig {
  agent?: AgentConfig;
  metrics?: MetricsConfig;
  logs?: LogsConfig;
}

export interface AgentConfig {
  region?: string;
  metrics_collection_interval?: number;
  debug?: boolean;
}

export interface MetricsConfig {
  namespace?: string;
  append_dimensions?: Record<string, string>;
  metrics_collected: MetricsCollected;
}

export interface MetricsCollected {
  cpu?: CPUMetrics;
  mem?: MemoryMetrics;
  disk?: DiskMetrics;
  diskio?: DiskIOMetrics;
  net?: NetworkMetrics;
  netstat?: NetstatMetrics;
  processes?: ProcessMetrics;
  [key: string]: any; // For OS-specific metrics
}

export interface CPUMetrics {
  measurement?: string[];
  metrics_collection_interval?: number;
  totalcpu?: boolean;
}

export interface MemoryMetrics {
  measurement?: string[];
  metrics_collection_interval?: number;
}

export interface DiskMetrics {
  measurement?: string[];
  metrics_collection_interval?: number;
  resources?: string[];
}

export interface DiskIOMetrics {
  measurement?: string[];
  metrics_collection_interval?: number;
  resources?: string[];
}

export interface NetworkMetrics {
  measurement?: string[];
  metrics_collection_interval?: number;
  resources?: string[];
}

export interface NetstatMetrics {
  measurement?: string[];
  metrics_collection_interval?: number;
}

export interface ProcessMetrics {
  measurement?: string[];
  metrics_collection_interval?: number;
}

export interface LogsConfig {
  logs_collected: {
    files?: FileLogsConfig;
    windows_events?: WindowsEventLogsConfig;
  };
  log_stream_name?: string;
}

export interface FileLogsConfig {
  collect_list: LogFileEntry[];
}

export interface WindowsEventLogsConfig {
  collect_list: WindowsEventLogEntry[];
}

export interface WindowsEventLogEntry {
  event_name: string;
  event_levels: string[];
  log_group_name: string;
  log_stream_name: string;
}

export interface LogFileEntry {
  file_path: string;
  log_group_name: string;
  log_stream_name: string;
  timezone?: string;
  filters?: LogFilter[];
  multiline_start_pattern?: string;
  timestamp_format?: string;
}

export interface LogFilter {
  type: 'include' | 'exclude';
  expression: string;
}

export interface ValidationError {
  field: string;
  message: string;
  path?: string;
}

export interface ConfigTemplate {
  id: string;
  name: string;
  description?: string;
  createdAt: Date;
  updatedAt: Date;
  operatingSystem: 'linux' | 'windows' | 'darwin';
  configuration: CloudWatchConfig;
}

export type OperatingSystem = 'linux' | 'windows' | 'darwin';