import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useValidation, useFieldValidation, useFormValidation } from '../useValidation';
import { CloudWatchConfig } from '../../types/config';

// Mock the validation utilities
vi.mock('../../utils/validation', () => ({
  validateCloudWatchConfig: vi.fn(),
  validateField: vi.fn(),
  isConfigurationValid: vi.fn(),
  getValidationErrorsByField: vi.fn()
}));

import { 
  validateCloudWatchConfig, 
  validateField, 
  isConfigurationValid,
  getValidationErrorsByField 
} from '../../utils/validation';

describe('useValidation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  const mockConfig: CloudWatchConfig = {
    metrics: {
      metrics_collected: {
        cpu: {
          measurement: ['cpu_usage_idle']
        }
      }
    }
  };

  const mockErrors = [
    { field: 'CPU', message: 'Invalid measurement', path: '/metrics/cpu' }
  ];

  describe('basic validation', () => {
    it('should initialize with empty errors', () => {
      (validateCloudWatchConfig as any).mockReturnValue([]);
      (getValidationErrorsByField as any).mockReturnValue({});

      const { result } = renderHook(() => 
        useValidation(mockConfig, { realTime: false })
      );

      expect(result.current.errors).toEqual([]);
      expect(result.current.isValid).toBe(true);
      expect(result.current.errorsByField).toEqual({});
    });

    it('should validate configuration manually', () => {
      (validateCloudWatchConfig as any).mockReturnValue(mockErrors);
      (getValidationErrorsByField as any).mockReturnValue({ CPU: mockErrors });

      const { result } = renderHook(() => 
        useValidation(mockConfig, { realTime: false })
      );

      act(() => {
        const errors = result.current.validate();
        expect(errors).toEqual(mockErrors);
      });

      expect(validateCloudWatchConfig).toHaveBeenCalledWith(mockConfig, undefined);
    });

    it('should validate with operating system', () => {
      (validateCloudWatchConfig as any).mockReturnValue([]);

      const { result } = renderHook(() => 
        useValidation(mockConfig, { realTime: false, operatingSystem: 'linux' })
      );

      act(() => {
        result.current.validate();
      });

      expect(validateCloudWatchConfig).toHaveBeenCalledWith(mockConfig, 'linux');
    });

    it('should validate specific field', () => {
      const fieldErrors = [{ field: 'Region', message: 'Invalid', path: '/agent/region' }];
      (validateField as any).mockReturnValue(fieldErrors);

      const { result } = renderHook(() => 
        useValidation(mockConfig, { realTime: false })
      );

      act(() => {
        const errors = result.current.validateField('/agent/region', 'invalid');
        expect(errors).toEqual(fieldErrors);
      });

      expect(validateField).toHaveBeenCalledWith(mockConfig, '/agent/region', 'invalid', undefined);
    });

    it('should clear errors', () => {
      (validateCloudWatchConfig as any).mockReturnValue(mockErrors);
      (getValidationErrorsByField as any).mockReturnValue({ CPU: mockErrors });

      const { result } = renderHook(() => 
        useValidation(mockConfig, { realTime: false })
      );

      act(() => {
        result.current.validate();
      });

      act(() => {
        result.current.clearErrors();
      });

      expect(result.current.errors).toEqual([]);
    });

    it('should get field errors', () => {
      (validateCloudWatchConfig as any).mockReturnValue(mockErrors);

      const { result } = renderHook(() => 
        useValidation(mockConfig, { realTime: false })
      );

      act(() => {
        result.current.validate();
      });

      const fieldErrors = result.current.getFieldErrors('CPU');
      expect(fieldErrors).toEqual(mockErrors);
    });
  });

  describe('real-time validation', () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it('should trigger validation on config change with debouncing', () => {
      (validateCloudWatchConfig as any).mockReturnValue([]);

      const { rerender } = renderHook(
        ({ config }) => useValidation(config, { realTime: true, debounceMs: 100 }),
        { initialProps: { config: mockConfig } }
      );

      expect(validateCloudWatchConfig).not.toHaveBeenCalled();

      // Fast forward past debounce delay
      act(() => {
        vi.advanceTimersByTime(100);
      });

      expect(validateCloudWatchConfig).toHaveBeenCalledTimes(1);

      // Change config
      const newConfig = { ...mockConfig, agent: { region: 'us-east-1' } };
      rerender({ config: newConfig });

      // Should not validate immediately
      expect(validateCloudWatchConfig).toHaveBeenCalledTimes(1);

      // Fast forward past debounce delay
      act(() => {
        vi.advanceTimersByTime(100);
      });

      expect(validateCloudWatchConfig).toHaveBeenCalledTimes(2);
      expect(validateCloudWatchConfig).toHaveBeenLastCalledWith(newConfig, undefined);
    });

    it('should not trigger real-time validation when disabled', () => {
      (validateCloudWatchConfig as any).mockReturnValue([]);

      renderHook(() => useValidation(mockConfig, { realTime: false }));

      act(() => {
        vi.advanceTimersByTime(1000);
      });

      expect(validateCloudWatchConfig).not.toHaveBeenCalled();
    });

    it('should debounce rapid config changes', () => {
      (validateCloudWatchConfig as any).mockReturnValue([]);

      const { rerender } = renderHook(
        ({ config }) => useValidation(config, { realTime: true, debounceMs: 100 }),
        { initialProps: { config: mockConfig } }
      );

      // Make rapid changes
      for (let i = 0; i < 5; i++) {
        const newConfig = { ...mockConfig, agent: { region: `region-${i}` } };
        rerender({ config: newConfig });
        vi.advanceTimersByTime(50); // Less than debounce delay
      }

      // Should not have validated yet
      expect(validateCloudWatchConfig).not.toHaveBeenCalled();

      // Fast forward past debounce delay
      act(() => {
        vi.advanceTimersByTime(100);
      });

      // Should validate only once with the final config
      expect(validateCloudWatchConfig).toHaveBeenCalledTimes(1);
    });
  });
});

describe('useFieldValidation', () => {
  const mockConfig: CloudWatchConfig = {
    metrics: {
      metrics_collected: {
        cpu: { measurement: ['cpu_usage_idle'] }
      }
    }
  };

  it('should validate field value', () => {
    const fieldErrors = [{ field: 'Region', message: 'Invalid', path: '/agent/region' }];
    (validateField as any).mockReturnValue(fieldErrors);

    const { result } = renderHook(() => 
      useFieldValidation(mockConfig, '/agent/region', 'linux')
    );

    const errors = result.current.validateField('invalid-region');
    expect(errors).toEqual(fieldErrors);
    expect(validateField).toHaveBeenCalledWith(mockConfig, '/agent/region', 'invalid-region', 'linux');
  });
});

describe('useFormValidation', () => {
  const mockConfig: CloudWatchConfig = {
    metrics: {
      metrics_collected: {
        cpu: { measurement: ['cpu_usage_idle'] }
      }
    }
  };

  it('should validate entire form', () => {
    (validateCloudWatchConfig as any).mockReturnValue([]);

    const { result } = renderHook(() => 
      useFormValidation(mockConfig, 'linux')
    );

    act(() => {
      const isValid = result.current.validateForm();
      expect(isValid).toBe(true);
    });

    expect(validateCloudWatchConfig).toHaveBeenCalledWith(mockConfig, 'linux');
  });

  it('should update config and validate', () => {
    (validateCloudWatchConfig as any).mockReturnValue([]);

    const { result } = renderHook(() => 
      useFormValidation(mockConfig, 'linux')
    );

    const newConfig = { ...mockConfig, agent: { region: 'us-east-1' } };

    act(() => {
      result.current.updateConfig(newConfig);
      result.current.validateForm();
    });

    expect(validateCloudWatchConfig).toHaveBeenCalledWith(newConfig, 'linux');
  });

  it('should validate individual field in form context', () => {
    const fieldErrors = [{ field: 'Region', message: 'Invalid', path: '/agent/region' }];
    (validateField as any).mockReturnValue(fieldErrors);

    const { result } = renderHook(() => 
      useFormValidation(mockConfig, 'linux')
    );

    const errors = result.current.validateField('/agent/region', 'invalid');
    expect(errors).toEqual(fieldErrors);
    expect(validateField).toHaveBeenCalledWith(mockConfig, '/agent/region', 'invalid', 'linux');
  });

  it('should check if form is valid', () => {
    (isConfigurationValid as any).mockReturnValue(true);

    const { result } = renderHook(() => 
      useFormValidation(mockConfig, 'linux')
    );

    const isValid = result.current.isValid();
    expect(isValid).toBe(true);
    expect(isConfigurationValid).toHaveBeenCalledWith(mockConfig, 'linux');
  });

  it('should get form errors', () => {
    const mockErrors = [{ field: 'CPU', message: 'Invalid', path: '/metrics/cpu' }];
    (validateCloudWatchConfig as any).mockReturnValue(mockErrors);

    const { result } = renderHook(() => 
      useFormValidation(mockConfig, 'linux')
    );

    act(() => {
      result.current.validateForm();
    });

    const errors = result.current.getErrors();
    expect(errors).toEqual(mockErrors);
  });
});