/**
 * CloudWatch Agent JSON Schema subset
 * Adapted from the official CloudWatch Agent configuration schema
 * Focuses on commonly used configurations for metrics and logs
 */

export const cloudWatchAgentSchema = {
  type: "object",
  properties: {
    agent: {
      type: "object",
      properties: {
        region: {
          type: "string",
          pattern: "^[a-z0-9-]+$",
          description: "AWS region where metrics will be sent"
        },
        metrics_collection_interval: {
          type: "integer",
          minimum: 1,
          maximum: 86400,
          description: "Default collection interval in seconds"
        },
        debug: {
          type: "boolean",
          description: "Enable debug logging"
        }
      },
      additionalProperties: false
    },
    metrics: {
      type: "object",
      properties: {
        namespace: {
          type: "string",
          minLength: 1,
          maxLength: 255,
          pattern: "^[a-zA-Z0-9._/#:-]+$",
          description: "CloudWatch namespace for custom metrics"
        },
        append_dimensions: {
          type: "object",
          patternProperties: {
            "^[a-zA-Z0-9._-]+$": {
              type: "string"
            }
          },
          additionalProperties: false,
          description: "Additional dimensions to append to all metrics"
        },
        metrics_collected: {
          type: "object",
          properties: {
            cpu: {
              type: "object",
              properties: {
                measurement: {
                  type: "array",
                  items: {
                    type: "string",
                    enum: [
                      "cpu_usage_idle",
                      "cpu_usage_iowait", 
                      "cpu_usage_user",
                      "cpu_usage_system",
                      "cpu_usage_steal",
                      "cpu_usage_nice",
                      "cpu_usage_softirq",
                      "cpu_usage_irq",
                      "cpu_usage_guest",
                      "cpu_usage_guest_nice",
                      "cpu_usage_active"
                    ]
                  },
                  minItems: 1,
                  uniqueItems: true
                },
                metrics_collection_interval: {
                  type: "integer",
                  minimum: 1,
                  maximum: 86400
                },
                totalcpu: {
                  type: "boolean",
                  description: "Collect total CPU metrics across all cores"
                }
              },
              additionalProperties: false
            },
            mem: {
              type: "object",
              properties: {
                measurement: {
                  type: "array",
                  items: {
                    type: "string",
                    enum: [
                      "mem_used",
                      "mem_cached",
                      "mem_total",
                      "mem_free",
                      "mem_used_percent",
                      "mem_available",
                      "mem_available_percent"
                    ]
                  },
                  minItems: 1,
                  uniqueItems: true
                },
                metrics_collection_interval: {
                  type: "integer",
                  minimum: 1,
                  maximum: 86400
                }
              },
              additionalProperties: false
            },
            disk: {
              type: "object",
              properties: {
                measurement: {
                  type: "array",
                  items: {
                    type: "string",
                    enum: [
                      "used_percent",
                      "inodes_free",
                      "inodes_used",
                      "inodes_total",
                      "used",
                      "free",
                      "total"
                    ]
                  },
                  minItems: 1,
                  uniqueItems: true
                },
                metrics_collection_interval: {
                  type: "integer",
                  minimum: 1,
                  maximum: 86400
                },
                resources: {
                  type: "array",
                  items: {
                    type: "string",
                    minLength: 1
                  },
                  minItems: 1,
                  uniqueItems: true,
                  description: "Disk mount points or drive letters to monitor"
                }
              },
              additionalProperties: false
            },
            diskio: {
              type: "object",
              properties: {
                measurement: {
                  type: "array",
                  items: {
                    type: "string",
                    enum: [
                      "reads",
                      "writes",
                      "read_bytes",
                      "write_bytes",
                      "read_time",
                      "write_time",
                      "io_time"
                    ]
                  },
                  minItems: 1,
                  uniqueItems: true
                },
                metrics_collection_interval: {
                  type: "integer",
                  minimum: 1,
                  maximum: 86400
                },
                resources: {
                  type: "array",
                  items: {
                    type: "string",
                    minLength: 1
                  },
                  minItems: 1,
                  uniqueItems: true
                }
              },
              additionalProperties: false
            },
            net: {
              type: "object",
              properties: {
                measurement: {
                  type: "array",
                  items: {
                    type: "string",
                    enum: [
                      "bytes_sent",
                      "bytes_recv",
                      "packets_sent",
                      "packets_recv",
                      "err_in",
                      "err_out",
                      "drop_in",
                      "drop_out"
                    ]
                  },
                  minItems: 1,
                  uniqueItems: true
                },
                metrics_collection_interval: {
                  type: "integer",
                  minimum: 1,
                  maximum: 86400
                },
                resources: {
                  type: "array",
                  items: {
                    type: "string",
                    minLength: 1
                  },
                  minItems: 1,
                  uniqueItems: true,
                  description: "Network interfaces to monitor"
                }
              },
              additionalProperties: false
            },
            netstat: {
              type: "object",
              properties: {
                measurement: {
                  type: "array",
                  items: {
                    type: "string",
                    enum: [
                      "tcp_established",
                      "tcp_syn_sent",
                      "tcp_syn_recv",
                      "tcp_fin_wait1",
                      "tcp_fin_wait2",
                      "tcp_time_wait",
                      "tcp_close",
                      "tcp_close_wait",
                      "tcp_last_ack",
                      "tcp_listen",
                      "tcp_closing",
                      "udp_socket"
                    ]
                  },
                  minItems: 1,
                  uniqueItems: true
                },
                metrics_collection_interval: {
                  type: "integer",
                  minimum: 1,
                  maximum: 86400
                }
              },
              additionalProperties: false
            },
            processes: {
              type: "object",
              properties: {
                measurement: {
                  type: "array",
                  items: {
                    type: "string",
                    enum: [
                      "running",
                      "sleeping",
                      "dead",
                      "zombies",
                      "stopped",
                      "total",
                      "total_threads"
                    ]
                  },
                  minItems: 1,
                  uniqueItems: true
                },
                metrics_collection_interval: {
                  type: "integer",
                  minimum: 1,
                  maximum: 86400
                }
              },
              additionalProperties: false
            }
          },
          additionalProperties: false,
          minProperties: 1
        }
      },
      required: ["metrics_collected"],
      additionalProperties: false
    },
    logs: {
      type: "object",
      properties: {
        logs_collected: {
          type: "object",
          properties: {
            files: {
              type: "object",
              properties: {
                collect_list: {
                  type: "array",
                  items: {
                    type: "object",
                    properties: {
                      file_path: {
                        type: "string",
                        minLength: 1,
                        description: "Path to the log file"
                      },
                      log_group_name: {
                        type: "string",
                        minLength: 1,
                        maxLength: 512,
                        pattern: "^[a-zA-Z0-9._/#:-]+$",
                        description: "CloudWatch log group name"
                      },
                      log_stream_name: {
                        type: "string",
                        minLength: 1,
                        maxLength: 512,
                        pattern: "^[a-zA-Z0-9._/#:-]+$",
                        description: "CloudWatch log stream name"
                      },
                      timezone: {
                        type: "string",
                        enum: ["UTC", "Local"],
                        description: "Timezone for timestamp parsing"
                      },
                      timestamp_format: {
                        type: "string",
                        minLength: 1,
                        description: "Timestamp format pattern"
                      },
                      multiline_start_pattern: {
                        type: "string",
                        minLength: 1,
                        description: "Regex pattern to identify start of multiline log entries"
                      },
                      filters: {
                        type: "array",
                        items: {
                          type: "object",
                          properties: {
                            type: {
                              type: "string",
                              enum: ["include", "exclude"]
                            },
                            expression: {
                              type: "string",
                              minLength: 1,
                              description: "Regular expression for filtering"
                            }
                          },
                          required: ["type", "expression"],
                          additionalProperties: false
                        }
                      }
                    },
                    required: ["file_path", "log_group_name", "log_stream_name"],
                    additionalProperties: false
                  },
                  minItems: 1
                }
              },
              required: ["collect_list"],
              additionalProperties: false
            },
            windows_events: {
              type: "object",
              properties: {
                collect_list: {
                  type: "array",
                  items: {
                    type: "object",
                    properties: {
                      event_name: {
                        type: "string",
                        enum: ["System", "Application", "Security"],
                        description: "Windows Event Log name"
                      },
                      event_levels: {
                        type: "array",
                        items: {
                          type: "string",
                          enum: ["INFORMATION", "WARNING", "ERROR", "CRITICAL", "VERBOSE"]
                        },
                        minItems: 1,
                        uniqueItems: true
                      },
                      log_group_name: {
                        type: "string",
                        minLength: 1,
                        maxLength: 512,
                        pattern: "^[a-zA-Z0-9._/#:-]+$"
                      },
                      log_stream_name: {
                        type: "string",
                        minLength: 1,
                        maxLength: 512,
                        pattern: "^[a-zA-Z0-9._/#:-]+$"
                      }
                    },
                    required: ["event_name", "event_levels", "log_group_name", "log_stream_name"],
                    additionalProperties: false
                  },
                  minItems: 1
                }
              },
              required: ["collect_list"],
              additionalProperties: false
            }
          },
          additionalProperties: false,
          minProperties: 1
        },
        log_stream_name: {
          type: "string",
          minLength: 1,
          maxLength: 512,
          pattern: "^[a-zA-Z0-9._/#:-]+$",
          description: "Default log stream name"
        }
      },
      required: ["logs_collected"],
      additionalProperties: false
    }
  },
  additionalProperties: false,
  anyOf: [
    { required: ["metrics"] },
    { required: ["logs"] }
  ]
} as const;

// OS-specific validation rules
export const osSpecificRules = {
  windows: {
    // Windows-specific metric measurements
    cpu: ["% Processor Time", "% User Time", "% Privileged Time", "% Interrupt Time"],
    memory: ["% Committed Bytes In Use", "Available MBytes", "Cache Faults/sec"],
    disk: ["% Free Space", "Free Megabytes", "% Disk Time"],
    network: ["Bytes Sent/sec", "Bytes Received/sec", "Packets/sec"]
  },
  linux: {
    // Linux-specific validations (file paths, etc.)
    logFilePath: "^(/[^/\\0]+)+/?$",
    diskResources: "^/[^\\0]*$"
  },
  darwin: {
    // macOS-specific validations
    logFilePath: "^(/[^/\\0]+)+/?$",
    diskResources: "^/[^\\0]*$"
  }
};