import { useCallback, useEffect, useMemo, useRef } from 'react';
import { CloudWatchConfig, ValidationError, OperatingSystem } from '../types/config';
import { 
  validateCloudWatchConfig, 
  validateField, 
  isConfigurationValid,
  getValidationErrorsByField 
} from '../utils/validation';

interface UseValidationOptions {
  /** Enable real-time validation on config changes */
  realTime?: boolean;
  /** Debounce delay for real-time validation in milliseconds */
  debounceMs?: number;
  /** Operating system for OS-specific validation */
  operatingSystem?: OperatingSystem;
}

interface UseValidationReturn {
  /** Current validation errors */
  errors: ValidationError[];
  /** Whether the configuration is valid */
  isValid: boolean;
  /** Errors grouped by field name */
  errorsByField: Record<string, ValidationError[]>;
  /** Manually trigger validation */
  validate: () => ValidationError[];
  /** Validate a specific field */
  validateField: (fieldPath: string, value: any) => ValidationError[];
  /** Clear all validation errors */
  clearErrors: () => void;
  /** Get errors for a specific field */
  getFieldErrors: (fieldName: string) => ValidationError[];
}

export function useValidation(
  config: CloudWatchConfig,
  options: UseValidationOptions = {}
): UseValidationReturn {
  const {
    realTime = true,
    debounceMs = 300,
    operatingSystem
  } = options;

  const errorsRef = useRef<ValidationError[]>([]);
  const debounceTimeoutRef = useRef<NodeJS.Timeout>();

  // Memoized validation function
  const validate = useCallback((): ValidationError[] => {
    const newErrors = validateCloudWatchConfig(config, operatingSystem);
    errorsRef.current = newErrors;
    return newErrors;
  }, [config, operatingSystem]);

  // Validate specific field
  const validateFieldCallback = useCallback((fieldPath: string, value: any): ValidationError[] => {
    return validateField(config, fieldPath, value, operatingSystem);
  }, [config, operatingSystem]);

  // Clear errors
  const clearErrors = useCallback(() => {
    errorsRef.current = [];
  }, []);

  // Get errors for specific field
  const getFieldErrors = useCallback((fieldName: string): ValidationError[] => {
    return errorsRef.current.filter(error => error.field === fieldName);
  }, []);

  // Real-time validation with debouncing
  useEffect(() => {
    if (!realTime) return;

    // Clear existing timeout
    if (debounceTimeoutRef.current) {
      clearTimeout(debounceTimeoutRef.current);
    }

    // Set new timeout for debounced validation
    debounceTimeoutRef.current = setTimeout(() => {
      validate();
    }, debounceMs);

    // Cleanup timeout on unmount or dependency change
    return () => {
      if (debounceTimeoutRef.current) {
        clearTimeout(debounceTimeoutRef.current);
      }
    };
  }, [config, operatingSystem, realTime, debounceMs, validate]);

  // Memoized computed values
  const errors = useMemo(() => errorsRef.current, [errorsRef.current]);
  const isValid = useMemo(() => errors.length === 0, [errors]);
  const errorsByField = useMemo(() => getValidationErrorsByField(errors), [errors]);

  return {
    errors,
    isValid,
    errorsByField,
    validate,
    validateField: validateFieldCallback,
    clearErrors,
    getFieldErrors
  };
}

/**
 * Hook for validating individual form fields with real-time feedback
 */
export function useFieldValidation(
  config: CloudWatchConfig,
  fieldPath: string,
  operatingSystem?: OperatingSystem
) {
  const validateFieldValue = useCallback((value: any): ValidationError[] => {
    return validateField(config, fieldPath, value, operatingSystem);
  }, [config, fieldPath, operatingSystem]);

  return {
    validateField: validateFieldValue
  };
}

/**
 * Hook for validation state management in forms
 */
export function useFormValidation(
  initialConfig: CloudWatchConfig,
  operatingSystem?: OperatingSystem
) {
  const configRef = useRef(initialConfig);
  const errorsRef = useRef<ValidationError[]>([]);

  const updateConfig = useCallback((newConfig: CloudWatchConfig) => {
    configRef.current = newConfig;
  }, []);

  const validateForm = useCallback((): boolean => {
    const errors = validateCloudWatchConfig(configRef.current, operatingSystem);
    errorsRef.current = errors;
    return errors.length === 0;
  }, [operatingSystem]);

  const validateFieldInForm = useCallback((fieldPath: string, value: any): ValidationError[] => {
    return validateField(configRef.current, fieldPath, value, operatingSystem);
  }, [operatingSystem]);

  const getFormErrors = useCallback((): ValidationError[] => {
    return errorsRef.current;
  }, []);

  const isFormValid = useCallback((): boolean => {
    return isConfigurationValid(configRef.current, operatingSystem);
  }, [operatingSystem]);

  return {
    updateConfig,
    validateForm,
    validateField: validateFieldInForm,
    getErrors: getFormErrors,
    isValid: isFormValid
  };
}

/**
 * Hook for managing validation state across multiple steps/components
 */
export function useMultiStepValidation(
  config: CloudWatchConfig,
  operatingSystem?: OperatingSystem
) {
  const stepValidationRef = useRef<Record<string, ValidationError[]>>({});

  const validateStep = useCallback((stepName: string, stepConfig: Partial<CloudWatchConfig>): ValidationError[] => {
    // Create a merged config for this step
    const mergedConfig = { ...config, ...stepConfig };
    const errors = validateCloudWatchConfig(mergedConfig, operatingSystem);
    
    // Filter errors relevant to this step
    const stepErrors = errors.filter(error => {
      // This is a simplified approach - in a real implementation,
      // you might want to map specific paths to steps
      return error.path?.includes(stepName) || error.field.toLowerCase().includes(stepName.toLowerCase());
    });

    stepValidationRef.current[stepName] = stepErrors;
    return stepErrors;
  }, [config, operatingSystem]);

  const getStepErrors = useCallback((stepName: string): ValidationError[] => {
    return stepValidationRef.current[stepName] || [];
  }, []);

  const isStepValid = useCallback((stepName: string): boolean => {
    const stepErrors = stepValidationRef.current[stepName] || [];
    return stepErrors.length === 0;
  }, []);

  const getAllStepErrors = useCallback((): ValidationError[] => {
    return Object.values(stepValidationRef.current).flat();
  }, []);

  const clearStepErrors = useCallback((stepName: string) => {
    delete stepValidationRef.current[stepName];
  }, []);

  return {
    validateStep,
    getStepErrors,
    isStepValid,
    getAllStepErrors,
    clearStepErrors
  };
}