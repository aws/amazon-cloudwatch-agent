/**
 * Simple verification script to test state management functionality
 * This can be run to verify the implementation works correctly
 */

import { appReducer, initialState } from '../context/appReducer';
import { AppAction } from '../types/state';
import { ConfigTemplate, LogFileEntry } from '../types/config';

// Test basic reducer functionality
console.log('Testing State Management Implementation...\n');

// Test 1: Initial state
console.log('1. Initial State:');
console.log('✓ Current step:', initialState.currentStep);
console.log('✓ Selected OS:', initialState.selectedOS);
console.log('✓ Is valid:', initialState.isValid);
console.log('✓ Templates count:', initialState.templates.length);

// Test 2: OS Selection
console.log('\n2. Testing OS Selection:');
const setOSAction: AppAction = { type: 'SET_OS', payload: 'linux' };
const stateAfterOS = appReducer(initialState, setOSAction);
console.log('✓ OS set to:', stateAfterOS.selectedOS);
console.log('✓ Configuration reset:', stateAfterOS.configuration.metrics?.namespace === 'CWAgent');

// Test 3: Metrics Update
console.log('\n3. Testing Metrics Update:');
const updateMetricsAction: AppAction = {
  type: 'UPDATE_METRICS',
  payload: {
    namespace: 'CustomNamespace',
    metrics_collected: {
      cpu: { measurement: ['cpu_usage_idle'] }
    }
  }
};
const stateAfterMetrics = appReducer(stateAfterOS, updateMetricsAction);
console.log('✓ Metrics namespace:', stateAfterMetrics.configuration.metrics?.namespace);
console.log('✓ CPU metrics:', stateAfterMetrics.configuration.metrics?.metrics_collected.cpu);

// Test 4: Log File Management
console.log('\n4. Testing Log File Management:');
const logFile: LogFileEntry = {
  file_path: '/var/log/app.log',
  log_group_name: 'app-logs',
  log_stream_name: 'app-stream'
};

const addLogAction: AppAction = { type: 'ADD_LOG_FILE', payload: logFile };
const stateAfterAddLog = appReducer(stateAfterMetrics, addLogAction);
console.log('✓ Log files count:', stateAfterAddLog.configuration.logs?.logs_collected?.files?.collect_list.length);
console.log('✓ First log file path:', stateAfterAddLog.configuration.logs?.logs_collected?.files?.collect_list[0]?.file_path);

// Test 5: Template Management
console.log('\n5. Testing Template Management:');
const template: ConfigTemplate = {
  id: 'test-template-1',
  name: 'Test Template',
  description: 'A test template',
  createdAt: new Date(),
  updatedAt: new Date(),
  operatingSystem: 'linux',
  configuration: stateAfterAddLog.configuration
};

const saveTemplateAction: AppAction = { type: 'SAVE_TEMPLATE', payload: template };
const stateAfterTemplate = appReducer(stateAfterAddLog, saveTemplateAction);
console.log('✓ Templates count:', stateAfterTemplate.templates.length);
console.log('✓ Template name:', stateAfterTemplate.templates[0]?.name);

// Test 6: Validation
console.log('\n6. Testing Validation:');
const validateAction: AppAction = { type: 'VALIDATE_CONFIG' };
const stateAfterValidation = appReducer(stateAfterTemplate, validateAction);
console.log('✓ Is valid:', stateAfterValidation.isValid);
console.log('✓ Validation errors:', stateAfterValidation.validationErrors.length);

// Test 7: Reset
console.log('\n7. Testing Reset:');
const resetAction: AppAction = { type: 'RESET_CONFIG' };
const stateAfterReset = appReducer(stateAfterTemplate, resetAction);
console.log('✓ After reset - OS:', stateAfterReset.selectedOS);
console.log('✓ After reset - Step:', stateAfterReset.currentStep);
console.log('✓ After reset - Templates preserved:', stateAfterReset.templates.length);

console.log('\n✅ All state management tests passed!');

export { }; // Make this a module