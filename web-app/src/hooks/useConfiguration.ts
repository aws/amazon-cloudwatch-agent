import { useCallback } from 'react';
import { useAppContext } from '../context/AppContext';
import { OperatingSystem, MetricsConfig, LogsConfig, LogFileEntry, ConfigTemplate } from '../types/config';

/**
 * Custom hook for configuration state management
 */
export const useConfiguration = () => {
  const { state, dispatch } = useAppContext();

  // OS Selection
  const setOperatingSystem = useCallback((os: OperatingSystem) => {
    dispatch({ type: 'SET_OS', payload: os });
  }, [dispatch]);

  // Step Navigation
  const setCurrentStep = useCallback((step: number) => {
    dispatch({ type: 'SET_STEP', payload: step });
  }, [dispatch]);

  const nextStep = useCallback(() => {
    dispatch({ type: 'SET_STEP', payload: state.currentStep + 1 });
  }, [dispatch, state.currentStep]);

  const previousStep = useCallback(() => {
    dispatch({ type: 'SET_STEP', payload: Math.max(0, state.currentStep - 1) });
  }, [dispatch, state.currentStep]);

  // Metrics Configuration
  const updateMetrics = useCallback((metrics: Partial<MetricsConfig>) => {
    dispatch({ type: 'UPDATE_METRICS', payload: metrics });
  }, [dispatch]);

  // Logs Configuration
  const updateLogs = useCallback((logs: Partial<LogsConfig>) => {
    dispatch({ type: 'UPDATE_LOGS', payload: logs });
  }, [dispatch]);

  const addLogFile = useCallback((logFile: LogFileEntry) => {
    dispatch({ type: 'ADD_LOG_FILE', payload: logFile });
  }, [dispatch]);

  const removeLogFile = useCallback((index: number) => {
    dispatch({ type: 'REMOVE_LOG_FILE', payload: index });
  }, [dispatch]);

  const updateLogFile = useCallback((index: number, logFile: LogFileEntry) => {
    dispatch({ type: 'UPDATE_LOG_FILE', payload: { index, logFile } });
  }, [dispatch]);

  // Validation
  const validateConfiguration = useCallback(() => {
    dispatch({ type: 'VALIDATE_CONFIG' });
  }, [dispatch]);

  // Configuration Reset
  const resetConfiguration = useCallback(() => {
    dispatch({ type: 'RESET_CONFIG' });
  }, [dispatch]);

  return {
    // State
    currentStep: state.currentStep,
    selectedOS: state.selectedOS,
    configuration: state.configuration,
    validationErrors: state.validationErrors,
    isValid: state.isValid,

    // Actions
    setOperatingSystem,
    setCurrentStep,
    nextStep,
    previousStep,
    updateMetrics,
    updateLogs,
    addLogFile,
    removeLogFile,
    updateLogFile,
    validateConfiguration,
    resetConfiguration
  };
};