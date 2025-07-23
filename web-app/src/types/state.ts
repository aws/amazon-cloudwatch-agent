import { CloudWatchConfig, ConfigTemplate, ValidationError, OperatingSystem, LogFileEntry, MetricsConfig, LogsConfig } from './config';

export interface AppState {
  currentStep: number;
  selectedOS: OperatingSystem | null;
  configuration: CloudWatchConfig;
  validationErrors: ValidationError[];
  templates: ConfigTemplate[];
  isValid: boolean;
}

export type AppAction = 
  | { type: 'SET_OS'; payload: OperatingSystem }
  | { type: 'SET_STEP'; payload: number }
  | { type: 'UPDATE_METRICS'; payload: Partial<MetricsConfig> }
  | { type: 'UPDATE_LOGS'; payload: Partial<LogsConfig> }
  | { type: 'ADD_LOG_FILE'; payload: LogFileEntry }
  | { type: 'REMOVE_LOG_FILE'; payload: number }
  | { type: 'UPDATE_LOG_FILE'; payload: { index: number; logFile: LogFileEntry } }
  | { type: 'SET_VALIDATION_ERRORS'; payload: ValidationError[] }
  | { type: 'VALIDATE_CONFIG' }
  | { type: 'SAVE_TEMPLATE'; payload: ConfigTemplate }
  | { type: 'LOAD_TEMPLATE'; payload: string }
  | { type: 'DELETE_TEMPLATE'; payload: string }
  | { type: 'LOAD_TEMPLATES'; payload: ConfigTemplate[] }
  | { type: 'RESET_CONFIG' };