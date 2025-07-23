import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { ReactNode } from 'react';
import { AppProvider } from '../../context/AppContext';
import { useConfiguration } from '../useConfiguration';
import { LogFileEntry } from '../../types/config';

// Mock localStorage
vi.mock('../../utils/localStorage', () => ({
  saveToLocalStorage: vi.fn(),
  loadFromLocalStorage: vi.fn(() => []),
  removeFromLocalStorage: vi.fn(),
  clearAllConfigData: vi.fn(),
  isLocalStorageAvailable: vi.fn(() => true)
}));

const wrapper = ({ children }: { children: ReactNode }) => (
  <AppProvider>{children}</AppProvider>
);

describe('useConfiguration', () => {
  it('should provide initial state', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    expect(result.current.currentStep).toBe(0);
    expect(result.current.selectedOS).toBe(null);
    expect(result.current.configuration).toBeDefined();
    expect(result.current.validationErrors).toEqual([]);
    expect(result.current.isValid).toBe(false);
  });

  it('should set operating system', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    act(() => {
      result.current.setOperatingSystem('linux');
    });
    
    expect(result.current.selectedOS).toBe('linux');
  });

  it('should navigate steps', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    act(() => {
      result.current.nextStep();
    });
    
    expect(result.current.currentStep).toBe(1);
    
    act(() => {
      result.current.previousStep();
    });
    
    expect(result.current.currentStep).toBe(0);
    
    act(() => {
      result.current.setCurrentStep(3);
    });
    
    expect(result.current.currentStep).toBe(3);
  });

  it('should not go below step 0', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    act(() => {
      result.current.previousStep();
    });
    
    expect(result.current.currentStep).toBe(0);
  });

  it('should update metrics configuration', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    act(() => {
      result.current.updateMetrics({
        namespace: 'CustomNamespace',
        metrics_collected: {
          cpu: { measurement: ['cpu_usage_idle'] }
        }
      });
    });
    
    expect(result.current.configuration.metrics?.namespace).toBe('CustomNamespace');
    expect(result.current.configuration.metrics?.metrics_collected.cpu).toEqual({
      measurement: ['cpu_usage_idle']
    });
  });

  it('should update logs configuration', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    act(() => {
      result.current.updateLogs({
        log_stream_name: 'custom-stream'
      });
    });
    
    expect(result.current.configuration.logs?.log_stream_name).toBe('custom-stream');
  });

  it('should manage log files', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    const logFile: LogFileEntry = {
      file_path: '/var/log/app.log',
      log_group_name: 'app-logs',
      log_stream_name: 'app-stream'
    };
    
    // Add log file
    act(() => {
      result.current.addLogFile(logFile);
    });
    
    expect(result.current.configuration.logs?.logs_collected?.files?.collect_list).toHaveLength(1);
    expect(result.current.configuration.logs?.logs_collected?.files?.collect_list[0]).toEqual(logFile);
    
    // Update log file
    const updatedLogFile: LogFileEntry = {
      ...logFile,
      file_path: '/var/log/updated.log'
    };
    
    act(() => {
      result.current.updateLogFile(0, updatedLogFile);
    });
    
    expect(result.current.configuration.logs?.logs_collected?.files?.collect_list[0].file_path).toBe('/var/log/updated.log');
    
    // Remove log file
    act(() => {
      result.current.removeLogFile(0);
    });
    
    expect(result.current.configuration.logs?.logs_collected?.files?.collect_list).toHaveLength(0);
  });

  it('should validate configuration', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    // Initially invalid (no OS selected)
    act(() => {
      result.current.validateConfiguration();
    });
    
    expect(result.current.isValid).toBe(false);
    expect(result.current.validationErrors).toHaveLength(1);
    
    // Valid after selecting OS
    act(() => {
      result.current.setOperatingSystem('linux');
      result.current.validateConfiguration();
    });
    
    expect(result.current.isValid).toBe(true);
    expect(result.current.validationErrors).toHaveLength(0);
  });

  it('should reset configuration', () => {
    const { result } = renderHook(() => useConfiguration(), { wrapper });
    
    // Make some changes
    act(() => {
      result.current.setOperatingSystem('linux');
      result.current.setCurrentStep(2);
      result.current.updateMetrics({
        namespace: 'CustomNamespace'
      });
    });
    
    expect(result.current.selectedOS).toBe('linux');
    expect(result.current.currentStep).toBe(2);
    expect(result.current.configuration.metrics?.namespace).toBe('CustomNamespace');
    
    // Reset
    act(() => {
      result.current.resetConfiguration();
    });
    
    expect(result.current.selectedOS).toBe(null);
    expect(result.current.currentStep).toBe(0);
    expect(result.current.configuration.metrics?.namespace).toBe('CWAgent');
  });
});