# Journald Receiver

The journald receiver collects logs from the systemd journal on Linux systems and forwards them to Amazon CloudWatch Logs.

## Platform Support

**Linux only** - This receiver requires systemd and is only available on Linux systems. The CloudWatch Agent will return an error if journald configuration is present on non-Linux platforms.

## Prerequisites

- **systemd**: The system must be running systemd with journald enabled
- **journalctl**: The `journalctl` binary must be available in PATH (or configured via `journalctl_path`)
- **Permissions**: The CloudWatch Agent must have sufficient permissions to access the journal, typically by being a member of the `systemd-journal` group or running as root

## Configuration

Add the journald configuration under `logs.logs_collected.journald` in your CloudWatch Agent configuration file:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "units": ["ssh", "kubelet"],
        "priority": "info",
        "start_at": "end"
      }
    }
  }
}
```

## Configuration Options

### Basic Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `units` | array of strings | all units | Filter logs by systemd unit names (e.g., `["ssh", "kubelet", "docker"]`) |
| `priority` | string | `"info"` | Minimum log priority level: `emerg`, `alert`, `crit`, `err`, `warning`, `notice`, `info`, `debug` |
| `start_at` | string | `"end"` | Where to start reading: `beginning` (from start of journal) or `end` (only new logs) |

### Advanced Filtering

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `identifiers` | array of strings | none | Filter by `SYSLOG_IDENTIFIER` field values |
| `matches` | array of objects | none | Advanced field matching (e.g., `[{"_SYSTEMD_UNIT": "ssh.service"}]`) |
| `grep` | string | none | Regular expression to filter log messages (applied to MESSAGE field) |
| `dmesg` | boolean | `false` | Collect only kernel messages (dmesg) |

### File and Directory Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `directory` | string | system default | Journal directory path (default: `/run/log/journal` or `/var/log/journal`) |
| `files` | array of strings | none | Specific journal files to read instead of directory |
| `namespace` | string | none | Journal namespace to read from |

### Advanced Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `all` | boolean | `false` | Include long and unprintable log entries |
| `storage` | string | none | Storage extension ID for cursor persistence across agent restarts |
| `root_path` | string | none | Root path for chroot environments (e.g., containers) |
| `journalctl_path` | string | `journalctl` | Custom path to journalctl binary |
| `retry_on_failure` | object | none | Retry configuration for failed reads |

## Configuration Examples

### Example 1: Basic Collection

Collect all journal logs at info level and above:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "priority": "info",
        "start_at": "end"
      }
    }
  }
}
```

### Example 2: Specific Units

Collect logs only from SSH and Kubernetes services:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "units": ["ssh", "kubelet", "docker"],
        "priority": "warning",
        "start_at": "end"
      }
    }
  }
}
```

### Example 3: Kernel Messages Only

Collect only kernel messages (dmesg):

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "dmesg": true,
        "priority": "info",
        "start_at": "beginning"
      }
    }
  }
}
```

### Example 4: Advanced Filtering with Regex

Filter logs containing specific patterns:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "units": ["nginx", "apache2"],
        "grep": "error|warning|critical",
        "priority": "warning"
      }
    }
  }
}
```

### Example 5: Custom Directory and Identifiers

Read from a specific journal directory and filter by identifiers:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "directory": "/var/log/journal",
        "identifiers": ["systemd", "kernel"],
        "priority": "info",
        "start_at": "end"
      }
    }
  }
}
```

### Example 6: Advanced Field Matching

Use matches for complex filtering:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "matches": [
          {
            "_SYSTEMD_UNIT": "ssh.service",
            "PRIORITY": "6"
          }
        ],
        "start_at": "end"
      }
    }
  }
}
```

### Example 7: Container/Chroot Environment

Configure for containerized environments:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "root_path": "/host",
        "journalctl_path": "/usr/bin/journalctl",
        "units": ["docker", "containerd"],
        "priority": "info"
      }
    }
  }
}
```

## Log Output

Journald logs are sent to Amazon CloudWatch Logs with the following structure:

- **Log Group**: Configured in the CloudWatch Logs exporter section
- **Log Stream**: Typically includes instance ID or hostname
- **Log Fields**: Journal fields are preserved as structured log attributes

Common journal fields included:
- `MESSAGE`: The log message
- `PRIORITY`: Numeric priority (0-7)
- `_SYSTEMD_UNIT`: Systemd unit name
- `_PID`: Process ID
- `_HOSTNAME`: Hostname
- `_TRANSPORT`: Transport method (e.g., journal, syslog)
- `SYSLOG_IDENTIFIER`: Syslog identifier

## Priority Levels

Priority levels follow syslog severity standards:

| Priority | Value | Description |
|----------|-------|-------------|
| `emerg` | 0 | System is unusable |
| `alert` | 1 | Action must be taken immediately |
| `crit` | 2 | Critical conditions |
| `err` | 3 | Error conditions |
| `warning` | 4 | Warning conditions |
| `notice` | 5 | Normal but significant condition |
| `info` | 6 | Informational messages |
| `debug` | 7 | Debug-level messages |

When you set `priority: "info"`, logs at info level and above (info, notice, warning, err, crit, alert, emerg) are collected.

## Permissions

The CloudWatch Agent needs appropriate permissions to read the journal:

### Option 1: Run as Root
```bash
sudo systemctl start amazon-cloudwatch-agent
```

### Option 2: Add to systemd-journal Group
```bash
sudo usermod -aG systemd-journal cwagent
```

### Option 3: Grant Specific Permissions
```bash
sudo setfacl -m u:cwagent:rx /var/log/journal
sudo setfacl -m u:cwagent:rx /run/log/journal
```

## Troubleshooting

### Agent fails to start with journald config on non-Linux systems

**Error**: `journald receiver is only supported on Linux`

**Solution**: Remove the journald configuration or only deploy this configuration to Linux instances.

### No logs appearing in CloudWatch Logs

**Possible causes**:
1. **Permissions**: Verify the agent has access to the journal
   ```bash
   sudo -u cwagent journalctl -n 10
   ```

2. **journalctl not found**: Ensure journalctl is in PATH or configure `journalctl_path`
   ```bash
   which journalctl
   ```

3. **No matching logs**: Check your filters (units, priority, grep) aren't too restrictive
   ```bash
   journalctl -u ssh -p info -n 10
   ```

4. **start_at setting**: If set to `end`, only new logs after agent start are collected

### High memory usage

**Cause**: Collecting too many logs or very verbose units

**Solutions**:
- Increase priority level (e.g., from `debug` to `info` or `warning`)
- Filter by specific units instead of collecting all
- Use `grep` to filter messages
- Configure batch processor limits in the pipeline

### Logs missing after agent restart

**Cause**: Cursor position not persisted

**Solution**: Configure storage extension for cursor persistence:
```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "units": ["ssh"],
        "storage": "file_storage"
      }
    }
  }
}
```

## Performance Considerations

- **Filtering**: Apply filters (units, priority, grep) to reduce log volume
- **start_at**: Use `end` for production to avoid processing historical logs
- **Batch processing**: Logs are batched before sending to CloudWatch Logs
- **Cursor persistence**: Enable storage extension to avoid reprocessing logs after restarts

## Integration with CloudWatch Logs

The journald receiver integrates with the CloudWatch Agent's logs pipeline:

```
journald → batch processor → CloudWatch Logs exporter → CloudWatch Logs
```

Configure the CloudWatch Logs destination in the main agent configuration:

```json
{
  "logs": {
    "logs_collected": {
      "journald": {
        "units": ["ssh"],
        "priority": "info"
      }
    },
    "log_stream_name": "{instance_id}",
    "log_group_name": "/aws/ec2/journald"
  }
}
```

## References

- [systemd journal documentation](https://www.freedesktop.org/software/systemd/man/systemd-journald.service.html)
- [journalctl man page](https://www.freedesktop.org/software/systemd/man/journalctl.html)
- [OpenTelemetry journald receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/journaldreceiver)
- [Amazon CloudWatch Logs documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/)
