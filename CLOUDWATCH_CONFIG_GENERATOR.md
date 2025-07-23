# CloudWatch Config Generator

## Overview

The CloudWatch Config Generator is a web-based tool designed to simplify the creation of Amazon CloudWatch Agent configuration files. It provides an intuitive, step-by-step interface that guides users through the process of configuring metrics collection, log monitoring, and other CloudWatch Agent features.

## Features

### 🖥️ Operating System Support
- **Windows**: Full support for Windows-specific metrics and Event Log monitoring
- **Linux**: Comprehensive Linux system metrics and log file monitoring
- **macOS**: macOS-specific metrics and log configuration

### 📊 Metrics Configuration
- **CPU Metrics**: Per-core monitoring, utilization thresholds, and usage patterns
- **Memory Metrics**: Available memory, used memory, and utilization percentages
- **Disk Metrics**: Disk usage, I/O statistics, and mount point monitoring
- **Network Metrics**: Bytes sent/received, packet statistics, and interface monitoring
- **Process Metrics**: Process-specific monitoring and resource usage

### 📝 Log Monitoring
- **File-based Logs**: Configure monitoring for application and system log files
- **Log Filtering**: Create include/exclude patterns using regular expressions
- **Multiline Support**: Handle multiline log entries with custom patterns
- **Windows Event Logs**: Monitor Windows Event Log channels (Windows only)
- **Custom Parsing**: Configure timestamp formats and timezone handling

### ✅ Validation & Quality Assurance
- **Real-time Validation**: Immediate feedback on configuration errors
- **Schema Compliance**: Ensures generated configurations match CloudWatch Agent requirements
- **Preview Mode**: Review generated JSON before download
- **Error Reporting**: Clear, actionable error messages with suggested fixes

### 💾 Template Management
- **Save Templates**: Store frequently used configurations for reuse
- **Load Templates**: Quickly apply saved configurations to new setups
- **Template Library**: Manage multiple templates with descriptions and metadata
- **Import/Export**: Share templates between team members

### ♿ Accessibility
- **Keyboard Navigation**: Full keyboard support for all interface elements
- **Screen Reader Support**: ARIA labels and semantic HTML structure
- **High Contrast**: Support for high contrast display modes
- **Responsive Design**: Works on desktop, tablet, and mobile devices

## Getting Started

### Prerequisites
- Modern web browser (Chrome 90+, Firefox 88+, Safari 14+, Edge 90+)
- Basic understanding of CloudWatch Agent concepts
- Knowledge of your system's log file locations and monitoring requirements

### Using the Config Generator

#### Step 1: Select Operating System
1. Choose your target operating system (Windows, Linux, or macOS)
2. Review OS-specific capabilities and limitations
3. Proceed to metrics configuration

#### Step 2: Configure Metrics
1. Select metric categories you want to monitor:
   - CPU utilization and performance
   - Memory usage and availability
   - Disk space and I/O operations
   - Network traffic and statistics
   - Process monitoring
2. Configure collection intervals and measurement units
3. Set up OS-specific metrics as needed

#### Step 3: Set Up Log Monitoring
1. Add log files you want to monitor:
   - Specify file paths (supports wildcards)
   - Configure log group and stream names
   - Set up log retention policies
2. Create log filters:
   - Include patterns for relevant log entries
   - Exclude patterns for noise reduction
   - Use regular expressions for complex filtering
3. Configure advanced options:
   - Multiline log handling
   - Custom timestamp formats
   - Timezone settings

#### Step 4: Review and Download
1. Preview the generated JSON configuration
2. Validate configuration against CloudWatch Agent schema
3. Make any necessary adjustments
4. Download the configuration file
5. Optionally save as a template for future use

## Configuration Examples

### Basic Linux Server Monitoring
```json
{
  "agent": {
    "metrics_collection_interval": 60,
    "run_as_user": "cwagent"
  },
  "metrics": {
    "namespace": "CWAgent",
    "metrics_collected": {
      "cpu": {
        "measurement": [
          "cpu_usage_idle",
          "cpu_usage_iowait",
          "cpu_usage_user",
          "cpu_usage_system"
        ],
        "metrics_collection_interval": 60,
        "totalcpu": false
      },
      "disk": {
        "measurement": [
          "used_percent"
        ],
        "metrics_collection_interval": 60,
        "resources": [
          "*"
        ]
      },
      "mem": {
        "measurement": [
          "mem_used_percent"
        ],
        "metrics_collection_interval": 60
      }
    }
  }
}
```

### Windows Server with Event Logs
```json
{
  "agent": {
    "metrics_collection_interval": 60
  },
  "metrics": {
    "namespace": "CWAgent",
    "metrics_collected": {
      "Memory": {
        "measurement": [
          "% Committed Bytes In Use"
        ],
        "metrics_collection_interval": 60
      },
      "Processor": {
        "measurement": [
          "% Processor Time"
        ],
        "metrics_collection_interval": 60,
        "resources": [
          "_Total"
        ]
      }
    }
  },
  "logs": {
    "logs_collected": {
      "windows_events": {
        "collect_list": [
          {
            "event_name": "System",
            "event_levels": [
              "ERROR",
              "WARNING"
            ],
            "log_group_name": "windows-system-events",
            "log_stream_name": "{hostname}"
          }
        ]
      }
    }
  }
}
```

### Application Log Monitoring with Filters
```json
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/application/*.log",
            "log_group_name": "application-logs",
            "log_stream_name": "{hostname}-app",
            "timezone": "UTC",
            "filters": [
              {
                "type": "exclude",
                "expression": "DEBUG"
              },
              {
                "type": "include",
                "expression": "(ERROR|WARN|FATAL)"
              }
            ],
            "multiline_start_pattern": "^\\d{4}-\\d{2}-\\d{2}",
            "timestamp_format": "%Y-%m-%d %H:%M:%S"
          }
        ]
      }
    }
  }
}
```

## Advanced Features

### Custom Metric Namespaces
Configure custom namespaces to organize your metrics:
```json
{
  "metrics": {
    "namespace": "MyApplication/Production",
    "append_dimensions": {
      "Environment": "prod",
      "Service": "web-server"
    }
  }
}
```

### Log Aggregation Patterns
Set up complex log aggregation for microservices:
```json
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/microservice-*/app.log",
            "log_group_name": "microservices",
            "log_stream_name": "{hostname}-{service}",
            "filters": [
              {
                "type": "include",
                "expression": "\\[REQUEST\\].*status:[45]\\d{2}"
              }
            ]
          }
        ]
      }
    }
  }
}
```

## Troubleshooting

### Common Issues

#### Configuration Validation Errors
- **Invalid file paths**: Ensure file paths exist and are accessible
- **Regex syntax errors**: Test regular expressions before applying
- **Missing required fields**: All required configuration sections must be present

#### Template Management
- **Storage limitations**: Browser localStorage has size limits
- **Template corruption**: Export important templates as backup
- **Cross-browser compatibility**: Templates are browser-specific

#### Performance Considerations
- **Collection intervals**: Balance monitoring frequency with performance impact
- **Log volume**: High-volume logs may impact system performance
- **Metric cardinality**: Too many unique metric combinations can be costly

### Getting Help
- Review the [CloudWatch Agent documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html)
- Check the [troubleshooting guide](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/troubleshooting-CloudWatch-Agent.html)
- Validate configurations using the CloudWatch Agent config wizard

## Development Status

The CloudWatch Config Generator is currently in development. Key milestones:

- ✅ Requirements and design completed
- 🔄 Core functionality implementation in progress
- ⏳ Beta testing planned
- ⏳ Production release pending

For detailed implementation progress, see the [project specification](.kiro/specs/cloudwatch-config-generator/).

## Contributing

This tool is part of the Amazon CloudWatch Agent project. For contribution guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.