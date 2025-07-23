import Ajv, { ErrorObject } from 'ajv';
import addFormats from 'ajv-formats';
import { CloudWatchConfig, ValidationError, OperatingSystem } from '../types/config';
import { cloudWatchAgentSchema, osSpecificRules } from '../schemas/cloudwatch-agent-schema';

// Initialize Ajv with formats
const ajv = new Ajv({ 
  allErrors: true, 
  verbose: true,
  strict: false,
  removeAdditional: false
});
addFormats(ajv);

// Compile the schema
const validateConfig = ajv.compile(cloudWatchAgentSchema);

/**
 * Validates a CloudWatch configuration against the schema
 */
export function validateCloudWatchConfig(
  config: CloudWatchConfig, 
  operatingSystem?: OperatingSystem
): ValidationError[] {
  const errors: ValidationError[] = [];

  // Basic schema validation
  const isValid = validateConfig(config);
  
  if (!isValid && validateConfig.errors) {
    errors.push(...convertAjvErrorsToValidationErrors(validateConfig.errors));
  }

  // OS-specific validation
  if (operatingSystem) {
    errors.push(...validateOSSpecificRules(config, operatingSystem));
  }

  // Custom business logic validation
  errors.push(...validateBusinessRules(config));

  return errors;
}

/**
 * Converts Ajv errors to our ValidationError format
 */
function convertAjvErrorsToValidationErrors(ajvErrors: ErrorObject[]): ValidationError[] {
  return ajvErrors.map(error => {
    const field = getFieldNameFromPath(error.instancePath);
    const message = formatErrorMessage(error);
    
    return {
      field,
      message,
      path: error.instancePath
    };
  });
}

/**
 * Extracts a user-friendly field name from JSON path
 */
function getFieldNameFromPath(path: string): string {
  if (!path) return 'root';
  
  const segments = path.split('/').filter(Boolean);
  const lastSegment = segments[segments.length - 1];
  
  // Convert snake_case to readable format
  return lastSegment
    .replace(/_/g, ' ')
    .replace(/\b\w/g, l => l.toUpperCase());
}

/**
 * Formats Ajv error messages to be more user-friendly
 */
function formatErrorMessage(error: ErrorObject): string {
  const { keyword, message, params } = error;
  
  switch (keyword) {
    case 'required':
      return `${params?.missingProperty} is required`;
    case 'type':
      return `Expected ${params?.type} but received ${typeof error.data}`;
    case 'minimum':
      return `Value must be at least ${params?.limit}`;
    case 'maximum':
      return `Value must be at most ${params?.limit}`;
    case 'minLength':
      return `Must be at least ${params?.limit} characters long`;
    case 'maxLength':
      return `Must be at most ${params?.limit} characters long`;
    case 'pattern':
      return `Invalid format. ${message}`;
    case 'enum':
      return `Must be one of: ${params?.allowedValues?.join(', ')}`;
    case 'minItems':
      return `Must have at least ${params?.limit} items`;
    case 'uniqueItems':
      return 'Items must be unique';
    case 'additionalProperties':
      return `Unknown property: ${params?.additionalProperty}`;
    default:
      return message || 'Invalid value';
  }
}

/**
 * Validates OS-specific rules
 */
function validateOSSpecificRules(
  config: CloudWatchConfig, 
  os: OperatingSystem
): ValidationError[] {
  const errors: ValidationError[] = [];

  // Validate log file paths for Unix-like systems
  if ((os === 'linux' || os === 'darwin') && config.logs?.logs_collected?.files) {
    const filePathPattern = osSpecificRules[os].logFilePath;
    const regex = new RegExp(filePathPattern);
    
    config.logs.logs_collected.files.collect_list.forEach((logFile, index) => {
      if (!regex.test(logFile.file_path)) {
        errors.push({
          field: `Log File ${index + 1} Path`,
          message: `Invalid file path format for ${os}. Must be an absolute path.`,
          path: `/logs/logs_collected/files/collect_list/${index}/file_path`
        });
      }
    });
  }

  // Validate disk resources for Unix-like systems
  if ((os === 'linux' || os === 'darwin') && config.metrics?.metrics_collected?.disk?.resources) {
    const diskPattern = osSpecificRules[os].diskResources;
    const regex = new RegExp(diskPattern);
    
    config.metrics.metrics_collected.disk.resources.forEach((resource, index) => {
      if (!regex.test(resource)) {
        errors.push({
          field: `Disk Resource ${index + 1}`,
          message: `Invalid disk path format for ${os}. Must be an absolute path.`,
          path: `/metrics/metrics_collected/disk/resources/${index}`
        });
      }
    });
  }

  // Validate Windows Event Log configuration
  if (os === 'windows' && config.logs?.logs_collected?.windows_events) {
    // Windows Event Logs are only valid on Windows
  } else if (os !== 'windows' && config.logs?.logs_collected?.windows_events) {
    errors.push({
      field: 'Windows Events',
      message: 'Windows Event Log collection is only available on Windows operating systems',
      path: '/logs/logs_collected/windows_events'
    });
  }

  return errors;
}

/**
 * Validates custom business rules
 */
function validateBusinessRules(config: CloudWatchConfig): ValidationError[] {
  const errors: ValidationError[] = [];

  // Ensure at least one metric or log is configured
  const hasMetrics = config.metrics?.metrics_collected && 
    Object.keys(config.metrics.metrics_collected).length > 0;
  const hasLogs = config.logs?.logs_collected && (
    config.logs.logs_collected.files?.collect_list?.length > 0 ||
    config.logs.logs_collected.windows_events?.collect_list?.length > 0
  );

  if (!hasMetrics && !hasLogs) {
    errors.push({
      field: 'Configuration',
      message: 'At least one metric type or log file must be configured',
      path: ''
    });
  }

  // Validate log filter regex patterns
  if (config.logs?.logs_collected?.files) {
    config.logs.logs_collected.files.collect_list.forEach((logFile, fileIndex) => {
      if (logFile.filters) {
        logFile.filters.forEach((filter, filterIndex) => {
          try {
            new RegExp(filter.expression);
          } catch (e) {
            errors.push({
              field: `Log File ${fileIndex + 1} Filter ${filterIndex + 1}`,
              message: `Invalid regular expression: ${filter.expression}`,
              path: `/logs/logs_collected/files/collect_list/${fileIndex}/filters/${filterIndex}/expression`
            });
          }
        });
      }

      // Validate multiline start pattern
      if (logFile.multiline_start_pattern) {
        try {
          new RegExp(logFile.multiline_start_pattern);
        } catch (e) {
          errors.push({
            field: `Log File ${fileIndex + 1} Multiline Pattern`,
            message: `Invalid regular expression: ${logFile.multiline_start_pattern}`,
            path: `/logs/logs_collected/files/collect_list/${fileIndex}/multiline_start_pattern`
          });
        }
      }
    });
  }

  // Validate metric collection intervals
  const validateInterval = (interval: number | undefined, path: string, fieldName: string) => {
    if (interval !== undefined && (interval < 1 || interval > 86400)) {
      errors.push({
        field: fieldName,
        message: 'Collection interval must be between 1 and 86400 seconds',
        path
      });
    }
  };

  if (config.metrics?.metrics_collected) {
    const metrics = config.metrics.metrics_collected;
    
    Object.entries(metrics).forEach(([metricType, metricConfig]) => {
      if (metricConfig && typeof metricConfig === 'object' && 'metrics_collection_interval' in metricConfig) {
        validateInterval(
          metricConfig.metrics_collection_interval,
          `/metrics/metrics_collected/${metricType}/metrics_collection_interval`,
          `${metricType} Collection Interval`
        );
      }
    });
  }

  return errors;
}

/**
 * Validates a specific field in real-time
 */
export function validateField(
  config: CloudWatchConfig,
  fieldPath: string,
  value: any,
  operatingSystem?: OperatingSystem
): ValidationError[] {
  // Create a minimal config with just the field being validated
  const testConfig = createTestConfigForField(config, fieldPath, value);
  
  // Run full validation but filter to only errors related to this field
  const allErrors = validateCloudWatchConfig(testConfig, operatingSystem);
  
  return allErrors.filter(error => 
    error.path === fieldPath || error.path?.startsWith(fieldPath + '/')
  );
}

/**
 * Creates a test configuration for field-specific validation
 */
function createTestConfigForField(
  baseConfig: CloudWatchConfig,
  fieldPath: string,
  value: any
): CloudWatchConfig {
  const testConfig = JSON.parse(JSON.stringify(baseConfig));
  
  // Set the specific field value in the test config
  const pathSegments = fieldPath.split('/').filter(Boolean);
  let current = testConfig;
  
  for (let i = 0; i < pathSegments.length - 1; i++) {
    const segment = pathSegments[i];
    if (!(segment in current)) {
      current[segment] = {};
    }
    current = current[segment];
  }
  
  const lastSegment = pathSegments[pathSegments.length - 1];
  if (lastSegment) {
    current[lastSegment] = value;
  }
  
  return testConfig;
}

/**
 * Checks if a configuration is valid
 */
export function isConfigurationValid(
  config: CloudWatchConfig,
  operatingSystem?: OperatingSystem
): boolean {
  const errors = validateCloudWatchConfig(config, operatingSystem);
  return errors.length === 0;
}

/**
 * Gets validation errors grouped by field
 */
export function getValidationErrorsByField(
  errors: ValidationError[]
): Record<string, ValidationError[]> {
  return errors.reduce((acc, error) => {
    const field = error.field;
    if (!acc[field]) {
      acc[field] = [];
    }
    acc[field].push(error);
    return acc;
  }, {} as Record<string, ValidationError[]>);
}