import { describe, it, expect } from 'vitest';
import { 
  validateCloudWatchConfig, 
  validateField, 
  isConfigurationValid,
  getValidationErrorsByField 
} from '../validation';
import { CloudWatchConfig, OperatingSystem } from '../../types/config';

describe('CloudWatch Configuration Validation', () => {
  describe('validateCloudWatchConfig', () => {
    it('should validate a minimal valid configuration', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle'],
              metrics_collection_interval: 60
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should require at least metrics or logs', () => {
      const config: CloudWatchConfig = {};

      const errors = validateCloudWatchConfig(config);
      expect(errors.length).toBeGreaterThan(0);
      expect(errors.some(e => e.message.includes('at least one metric type or log file'))).toBe(true);
    });

    it('should validate agent configuration', () => {
      const config: CloudWatchConfig = {
        agent: {
          region: 'us-east-1',
          metrics_collection_interval: 60,
          debug: true
        },
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle']
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should reject invalid agent region format', () => {
      const config: CloudWatchConfig = {
        agent: {
          region: 'INVALID_REGION!',
          metrics_collection_interval: 60
        },
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle']
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors.some(e => e.field === 'Region')).toBe(true);
    });

    it('should validate metrics collection interval bounds', () => {
      const config: CloudWatchConfig = {
        agent: {
          metrics_collection_interval: 0 // Invalid: too low
        },
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle'],
              metrics_collection_interval: 86401 // Invalid: too high
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors.some(e => e.message.includes('at least 1'))).toBe(true);
      expect(errors.some(e => e.message.includes('at most 86400'))).toBe(true);
    });

    it('should validate CPU metrics configuration', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle', 'cpu_usage_user'],
              metrics_collection_interval: 60,
              totalcpu: true
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should reject invalid CPU measurements', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['invalid_measurement']
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors.some(e => e.message.includes('Must be one of'))).toBe(true);
    });

    it('should validate memory metrics configuration', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            mem: {
              measurement: ['mem_used_percent', 'mem_available'],
              metrics_collection_interval: 30
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should validate disk metrics with resources', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            disk: {
              measurement: ['used_percent', 'free'],
              resources: ['/dev/sda1', '/dev/sdb1'],
              metrics_collection_interval: 60
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should require unique measurements', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle', 'cpu_usage_idle'] // Duplicate
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors.some(e => e.message.includes('unique'))).toBe(true);
    });
  });

  describe('Log Configuration Validation', () => {
    it('should validate file log configuration', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: '/var/log/app.log',
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-stream'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should validate log filters', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: '/var/log/app.log',
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-stream',
                  filters: [
                    {
                      type: 'include',
                      expression: 'ERROR'
                    },
                    {
                      type: 'exclude',
                      expression: 'DEBUG'
                    }
                  ]
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should reject invalid regex in filters', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: '/var/log/app.log',
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-stream',
                  filters: [
                    {
                      type: 'include',
                      expression: '[invalid regex'
                    }
                  ]
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors.some(e => e.message.includes('Invalid regular expression'))).toBe(true);
    });

    it('should validate multiline start pattern', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: '/var/log/app.log',
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-stream',
                  multiline_start_pattern: '^\\d{4}-\\d{2}-\\d{2}'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should reject invalid multiline pattern regex', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: '/var/log/app.log',
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-stream',
                  multiline_start_pattern: '[invalid'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors.some(e => e.message.includes('Invalid regular expression'))).toBe(true);
    });

    it('should validate Windows Event Log configuration', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            windows_events: {
              collect_list: [
                {
                  event_name: 'System',
                  event_levels: ['ERROR', 'WARNING'],
                  log_group_name: 'windows-system',
                  log_stream_name: 'system-events'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors).toHaveLength(0);
    });

    it('should reject invalid event levels', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            windows_events: {
              collect_list: [
                {
                  event_name: 'System',
                  event_levels: ['INVALID_LEVEL'],
                  log_group_name: 'windows-system',
                  log_stream_name: 'system-events'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config);
      expect(errors.some(e => e.message.includes('Must be one of'))).toBe(true);
    });
  });

  describe('OS-Specific Validation', () => {
    it('should validate Linux file paths', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: '/var/log/app.log',
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-stream'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config, 'linux');
      expect(errors).toHaveLength(0);
    });

    it('should reject invalid Linux file paths', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: 'relative/path.log', // Invalid: not absolute
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-stream'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config, 'linux');
      expect(errors.some(e => e.message.includes('absolute path'))).toBe(true);
    });

    it('should validate Linux disk resources', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            disk: {
              measurement: ['used_percent'],
              resources: ['/dev/sda1', '/']
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config, 'linux');
      expect(errors).toHaveLength(0);
    });

    it('should reject Windows Event Logs on non-Windows OS', () => {
      const config: CloudWatchConfig = {
        logs: {
          logs_collected: {
            windows_events: {
              collect_list: [
                {
                  event_name: 'System',
                  event_levels: ['ERROR'],
                  log_group_name: 'windows-system',
                  log_stream_name: 'system-events'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config, 'linux');
      expect(errors.some(e => e.message.includes('only available on Windows'))).toBe(true);
    });
  });

  describe('validateField', () => {
    it('should validate individual fields', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle']
            }
          }
        }
      };

      const errors = validateField(config, '/agent/region', 'us-east-1');
      expect(errors).toHaveLength(0);
    });

    it('should return errors for invalid field values', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle']
            }
          }
        }
      };

      const errors = validateField(config, '/agent/region', 'INVALID!');
      expect(errors.length).toBeGreaterThan(0);
    });
  });

  describe('isConfigurationValid', () => {
    it('should return true for valid configuration', () => {
      const config: CloudWatchConfig = {
        metrics: {
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle']
            }
          }
        }
      };

      expect(isConfigurationValid(config)).toBe(true);
    });

    it('should return false for invalid configuration', () => {
      const config: CloudWatchConfig = {};

      expect(isConfigurationValid(config)).toBe(false);
    });
  });

  describe('getValidationErrorsByField', () => {
    it('should group errors by field name', () => {
      const errors = [
        { field: 'Region', message: 'Invalid format', path: '/agent/region' },
        { field: 'Region', message: 'Required field', path: '/agent/region' },
        { field: 'CPU', message: 'Invalid measurement', path: '/metrics/cpu' }
      ];

      const grouped = getValidationErrorsByField(errors);

      expect(grouped['Region']).toHaveLength(2);
      expect(grouped['CPU']).toHaveLength(1);
    });
  });

  describe('Complex Configuration Validation', () => {
    it('should validate a complete configuration', () => {
      const config: CloudWatchConfig = {
        agent: {
          region: 'us-west-2',
          metrics_collection_interval: 60,
          debug: false
        },
        metrics: {
          namespace: 'MyApp/Metrics',
          append_dimensions: {
            Environment: 'production',
            Service: 'web-server'
          },
          metrics_collected: {
            cpu: {
              measurement: ['cpu_usage_idle', 'cpu_usage_user'],
              metrics_collection_interval: 30,
              totalcpu: true
            },
            mem: {
              measurement: ['mem_used_percent', 'mem_available']
            },
            disk: {
              measurement: ['used_percent', 'free'],
              resources: ['/dev/sda1', '/dev/sdb1']
            }
          }
        },
        logs: {
          logs_collected: {
            files: {
              collect_list: [
                {
                  file_path: '/var/log/app.log',
                  log_group_name: 'app-logs',
                  log_stream_name: 'app-{instance_id}',
                  timezone: 'UTC',
                  filters: [
                    {
                      type: 'include',
                      expression: 'ERROR|WARN'
                    }
                  ],
                  multiline_start_pattern: '^\\d{4}-\\d{2}-\\d{2}',
                  timestamp_format: '%Y-%m-%d %H:%M:%S'
                }
              ]
            }
          }
        }
      };

      const errors = validateCloudWatchConfig(config, 'linux');
      expect(errors).toHaveLength(0);
    });
  });
});