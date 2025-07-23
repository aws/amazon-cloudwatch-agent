import { describe, it, expect } from 'vitest';
import { appReducer, initialState } from '../appReducer';
import { AppState, AppAction } from '../../types/state';
import { ConfigTemplate, LogFileEntry } from '../../types/config';

describe('appReducer', () => {
  describe('SET_OS', () => {
    it('should set the operating system', () => {
      const action: AppAction = { type: 'SET_OS', payload: 'linux' };
      const newState = appReducer(initialState, action);
      
      expect(newState.selectedOS).toBe('linux');
    });

    it('should reset configuration when OS changes', () => {
      const stateWithConfig: AppState = {
        ...initialState,
        selectedOS: 'windows',
        configuration: {
          ...initialState.configuration,
          metrics: {
            namespace: 'CustomNamespace',
            metrics_collected: {
              cpu: { measurement: ['cpu_usage_idle'] }
            }
          }
        }
      };

      const action: AppAction = { type: 'SET_OS', payload: 'linux' };
      const newState = appReducer(stateWithConfig, action);
      
      expect(newState.selectedOS).toBe('linux');
      expect(newState.configuration.metrics?.namespace).toBe('CWAgent');
      expect(newState.configuration.metrics?.metrics_collected).toEqual({});
    });
  });

  describe('SET_STEP', () => {
    it('should set the current step', () => {
      const action: AppAction = { type: 'SET_STEP', payload: 2 };
      const newState = appReducer(initialState, action);
      
      expect(newState.currentStep).toBe(2);
    });
  });

  describe('UPDATE_METRICS', () => {
    it('should update metrics configuration', () => {
      const action: AppAction = {
        type: 'UPDATE_METRICS',
        payload: {
          namespace: 'CustomNamespace',
          metrics_collected: {
            cpu: { measurement: ['cpu_usage_idle', 'cpu_usage_user'] }
          }
        }
      };
      
      const newState = appReducer(initialState, action);
      
      expect(newState.configuration.metrics?.namespace).toBe('CustomNamespace');
      expect(newState.configuration.metrics?.metrics_collected.cpu).toEqual({
        measurement: ['cpu_usage_idle', 'cpu_usage_user']
      });
    });

    it('should merge with existing metrics configuration', () => {
      const stateWithMetrics: AppState = {
        ...initialState,
        configuration: {
          ...initialState.configuration,
          metrics: {
            namespace: 'ExistingNamespace',
            metrics_collected: {
              cpu: { measurement: ['cpu_usage_idle'] },
              mem: { measurement: ['mem_used_percent'] }
            }
          }
        }
      };

      const action: AppAction = {
        type: 'UPDATE_METRICS',
        payload: {
          metrics_collected: {
            cpu: { measurement: ['cpu_usage_user'] },
            disk: { measurement: ['used_percent'] }
          }
        }
      };
      
      const newState = appReducer(stateWithMetrics, action);
      
      expect(newState.configuration.metrics?.namespace).toBe('ExistingNamespace');
      expect(newState.configuration.metrics?.metrics_collected.cpu).toEqual({
        measurement: ['cpu_usage_user']
      });
      expect(newState.configuration.metrics?.metrics_collected.disk).toEqual({
        measurement: ['used_percent']
      });
    });
  });

  describe('ADD_LOG_FILE', () => {
    it('should add a log file to the configuration', () => {
      const logFile: LogFileEntry = {
        file_path: '/var/log/app.log',
        log_group_name: 'app-logs',
        log_stream_name: 'app-stream'
      };

      const action: AppAction = { type: 'ADD_LOG_FILE', payload: logFile };
      const newState = appReducer(initialState, action);
      
      expect(newState.configuration.logs?.logs_collected?.files?.collect_list).toHaveLength(1);
      expect(newState.configuration.logs?.logs_collected?.files?.collect_list[0]).toEqual(logFile);
    });

    it('should add to existing log files', () => {
      const existingLogFile: LogFileEntry = {
        file_path: '/var/log/existing.log',
        log_group_name: 'existing-logs',
        log_stream_name: 'existing-stream'
      };

      const stateWithLogFile: AppState = {
        ...initialState,
        configuration: {
          ...initialState.configuration,
          logs: {
            logs_collected: {
              files: {
                collect_list: [existingLogFile]
              }
            }
          }
        }
      };

      const newLogFile: LogFileEntry = {
        file_path: '/var/log/new.log',
        log_group_name: 'new-logs',
        log_stream_name: 'new-stream'
      };

      const action: AppAction = { type: 'ADD_LOG_FILE', payload: newLogFile };
      const newState = appReducer(stateWithLogFile, action);
      
      expect(newState.configuration.logs?.logs_collected?.files?.collect_list).toHaveLength(2);
      expect(newState.configuration.logs?.logs_collected?.files?.collect_list[1]).toEqual(newLogFile);
    });
  });

  describe('REMOVE_LOG_FILE', () => {
    it('should remove a log file by index', () => {
      const logFiles: LogFileEntry[] = [
        {
          file_path: '/var/log/app1.log',
          log_group_name: 'app1-logs',
          log_stream_name: 'app1-stream'
        },
        {
          file_path: '/var/log/app2.log',
          log_group_name: 'app2-logs',
          log_stream_name: 'app2-stream'
        }
      ];

      const stateWithLogFiles: AppState = {
        ...initialState,
        configuration: {
          ...initialState.configuration,
          logs: {
            logs_collected: {
              files: {
                collect_list: logFiles
              }
            }
          }
        }
      };

      const action: AppAction = { type: 'REMOVE_LOG_FILE', payload: 0 };
      const newState = appReducer(stateWithLogFiles, action);
      
      expect(newState.configuration.logs?.logs_collected?.files?.collect_list).toHaveLength(1);
      expect(newState.configuration.logs?.logs_collected?.files?.collect_list[0]).toEqual(logFiles[1]);
    });
  });

  describe('SAVE_TEMPLATE', () => {
    it('should add a new template', () => {
      const template: ConfigTemplate = {
        id: 'test-template-1',
        name: 'Test Template',
        description: 'A test template',
        createdAt: new Date(),
        updatedAt: new Date(),
        operatingSystem: 'linux',
        configuration: initialState.configuration
      };

      const action: AppAction = { type: 'SAVE_TEMPLATE', payload: template };
      const newState = appReducer(initialState, action);
      
      expect(newState.templates).toHaveLength(1);
      expect(newState.templates[0]).toEqual(template);
    });

    it('should update an existing template', () => {
      const originalTemplate: ConfigTemplate = {
        id: 'test-template-1',
        name: 'Original Template',
        description: 'Original description',
        createdAt: new Date(),
        updatedAt: new Date(),
        operatingSystem: 'linux',
        configuration: initialState.configuration
      };

      const stateWithTemplate: AppState = {
        ...initialState,
        templates: [originalTemplate]
      };

      const updatedTemplate: ConfigTemplate = {
        ...originalTemplate,
        name: 'Updated Template',
        description: 'Updated description'
      };

      const action: AppAction = { type: 'SAVE_TEMPLATE', payload: updatedTemplate };
      const newState = appReducer(stateWithTemplate, action);
      
      expect(newState.templates).toHaveLength(1);
      expect(newState.templates[0].name).toBe('Updated Template');
      expect(newState.templates[0].description).toBe('Updated description');
    });
  });

  describe('LOAD_TEMPLATE', () => {
    it('should load a template configuration', () => {
      const template: ConfigTemplate = {
        id: 'test-template-1',
        name: 'Test Template',
        createdAt: new Date(),
        updatedAt: new Date(),
        operatingSystem: 'windows',
        configuration: {
          agent: { debug: true },
          metrics: {
            namespace: 'TestNamespace',
            metrics_collected: {
              cpu: { measurement: ['cpu_usage_idle'] }
            }
          }
        }
      };

      const stateWithTemplate: AppState = {
        ...initialState,
        templates: [template]
      };

      const action: AppAction = { type: 'LOAD_TEMPLATE', payload: 'test-template-1' };
      const newState = appReducer(stateWithTemplate, action);
      
      expect(newState.selectedOS).toBe('windows');
      expect(newState.configuration.agent?.debug).toBe(true);
      expect(newState.configuration.metrics?.namespace).toBe('TestNamespace');
      expect(newState.currentStep).toBe(0);
    });

    it('should not change state if template not found', () => {
      const action: AppAction = { type: 'LOAD_TEMPLATE', payload: 'non-existent-template' };
      const newState = appReducer(initialState, action);
      
      expect(newState).toEqual(initialState);
    });
  });

  describe('DELETE_TEMPLATE', () => {
    it('should delete a template by id', () => {
      const templates: ConfigTemplate[] = [
        {
          id: 'template-1',
          name: 'Template 1',
          createdAt: new Date(),
          updatedAt: new Date(),
          operatingSystem: 'linux',
          configuration: initialState.configuration
        },
        {
          id: 'template-2',
          name: 'Template 2',
          createdAt: new Date(),
          updatedAt: new Date(),
          operatingSystem: 'windows',
          configuration: initialState.configuration
        }
      ];

      const stateWithTemplates: AppState = {
        ...initialState,
        templates
      };

      const action: AppAction = { type: 'DELETE_TEMPLATE', payload: 'template-1' };
      const newState = appReducer(stateWithTemplates, action);
      
      expect(newState.templates).toHaveLength(1);
      expect(newState.templates[0].id).toBe('template-2');
    });
  });

  describe('RESET_CONFIG', () => {
    it('should reset configuration to initial state but keep templates', () => {
      const templates: ConfigTemplate[] = [
        {
          id: 'template-1',
          name: 'Template 1',
          createdAt: new Date(),
          updatedAt: new Date(),
          operatingSystem: 'linux',
          configuration: initialState.configuration
        }
      ];

      const modifiedState: AppState = {
        currentStep: 3,
        selectedOS: 'windows',
        configuration: {
          agent: { debug: true },
          metrics: {
            namespace: 'CustomNamespace',
            metrics_collected: {
              cpu: { measurement: ['cpu_usage_idle'] }
            }
          }
        },
        validationErrors: [{ field: 'test', message: 'test error' }],
        templates,
        isValid: true
      };

      const action: AppAction = { type: 'RESET_CONFIG' };
      const newState = appReducer(modifiedState, action);
      
      expect(newState.currentStep).toBe(0);
      expect(newState.selectedOS).toBe(null);
      expect(newState.configuration).toEqual(initialState.configuration);
      expect(newState.validationErrors).toEqual([]);
      expect(newState.isValid).toBe(false);
      expect(newState.templates).toEqual(templates); // Templates should be preserved
    });
  });
});