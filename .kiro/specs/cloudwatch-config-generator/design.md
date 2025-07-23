# Design Document

## Overview

The CloudWatch Config Generator is a single-page web application that provides an intuitive interface for generating Amazon CloudWatch Agent configuration files. The application uses a step-by-step wizard approach to guide users through selecting their operating system, configuring metrics collection, setting up log monitoring with filters, and generating validated JSON configurations.

The application will be built as a client-side React application with TypeScript for type safety, utilizing a modular architecture that separates configuration logic, validation, and UI components. The generated configurations will be validated against the official CloudWatch Agent JSON schema to ensure compatibility.

## Architecture

### Frontend Architecture
- **Framework**: React 18 with TypeScript
- **State Management**: React Context API with useReducer for complex state
- **Styling**: Tailwind CSS for responsive design and accessibility
- **Validation**: JSON Schema validation using Ajv library
- **Build Tool**: Vite for fast development and optimized builds

### Component Architecture
```
App
├── ConfigurationWizard
│   ├── OSSelectionStep
│   ├── MetricsConfigurationStep
│   ├── LogsConfigurationStep
│   └── ReviewAndDownloadStep
├── ConfigurationPreview
├── TemplateManager
└── ValidationDisplay
```

### Data Flow
1. User selections flow through React Context
2. Configuration state is managed by a reducer
3. Real-time validation occurs on state changes
4. Final JSON generation happens before download

## Components and Interfaces

### Core Interfaces

```typescript
interface CloudWatchConfig {
  agent?: AgentConfig;
  metrics?: MetricsConfig;
  logs?: LogsConfig;
}

interface AgentConfig {
  region?: string;
  metrics_collection_interval?: number;
  debug?: boolean;
}

interface MetricsConfig {
  namespace?: string;
  append_dimensions?: Record<string, string>;
  metrics_collected: MetricsCollected;
}

interface MetricsCollected {
  cpu?: CPUMetrics;
  mem?: MemoryMetrics;
  disk?: DiskMetrics;
  diskio?: DiskIOMetrics;
  net?: NetworkMetrics;
  netstat?: NetstatMetrics;
  processes?: ProcessMetrics;
  [key: string]: any; // For OS-specific metrics
}

interface LogsConfig {
  logs_collected: {
    files?: FileLogsConfig;
    windows_events?: WindowsEventLogsConfig;
  };
  log_stream_name?: string;
}

interface FileLogsConfig {
  collect_list: LogFileEntry[];
}

interface LogFileEntry {
  file_path: string;
  log_group_name: string;
  log_stream_name: string;
  timezone?: string;
  filters?: LogFilter[];
  multiline_start_pattern?: string;
  timestamp_format?: string;
}

interface LogFilter {
  type: 'include' | 'exclude';
  expression: string;
}
```

### UI Components

#### OSSelectionStep
- Radio button group for OS selection (Linux, Windows, macOS)
- Dynamic help text explaining OS-specific capabilities
- Validation to ensure OS is selected before proceeding

#### MetricsConfigurationStep
- Categorized metric selection with expandable sections
- Per-metric configuration options (intervals, measurements)
- OS-specific metric filtering
- Real-time preview of selected metrics

#### LogsConfigurationStep
- File path input with validation
- Log group/stream name configuration
- Filter creation interface with regex validation
- Windows Event Log configuration (Windows only)
- Multiline log handling options

#### ConfigurationPreview
- Syntax-highlighted JSON display
- Collapsible sections for readability
- Validation status indicators
- Copy-to-clipboard functionality

#### TemplateManager
- Template list with search/filter
- Save/load/delete operations
- Template metadata (name, description, created date)
- Import/export functionality

### State Management

```typescript
interface AppState {
  currentStep: number;
  selectedOS: 'linux' | 'windows' | 'darwin' | null;
  configuration: CloudWatchConfig;
  validationErrors: ValidationError[];
  templates: ConfigTemplate[];
  isValid: boolean;
}

type AppAction = 
  | { type: 'SET_OS'; payload: string }
  | { type: 'UPDATE_METRICS'; payload: Partial<MetricsConfig> }
  | { type: 'UPDATE_LOGS'; payload: Partial<LogsConfig> }
  | { type: 'ADD_LOG_FILE'; payload: LogFileEntry }
  | { type: 'REMOVE_LOG_FILE'; payload: number }
  | { type: 'VALIDATE_CONFIG' }
  | { type: 'SAVE_TEMPLATE'; payload: ConfigTemplate }
  | { type: 'LOAD_TEMPLATE'; payload: string };
```

## Data Models

### Configuration Templates
Templates are stored in browser localStorage with the following structure:

```typescript
interface ConfigTemplate {
  id: string;
  name: string;
  description?: string;
  createdAt: Date;
  updatedAt: Date;
  operatingSystem: string;
  configuration: CloudWatchConfig;
}
```

### Validation Schema
The application includes a subset of the CloudWatch Agent JSON schema focused on commonly used configurations:

- Agent configuration validation
- Metrics collection validation with OS-specific rules
- Log file path and filter validation
- Required field validation
- Value range and format validation

### OS-Specific Configurations

#### Linux/macOS Metrics
- CPU: cpu_usage_idle, cpu_usage_iowait, cpu_usage_user, cpu_usage_system
- Memory: mem_used_percent, mem_available_percent
- Disk: used_percent, free, total
- Network: bytes_sent, bytes_recv, packets_sent, packets_recv

#### Windows Metrics
- Memory: "% Committed Bytes In Use"
- LogicalDisk: "% Free Space", "Free Megabytes"
- Processor: "% Processor Time", "% User Time"
- Network Interface: "Bytes Sent/sec", "Bytes Received/sec"

## Error Handling

### Validation Errors
- Real-time validation with immediate feedback
- Error messages linked to specific form fields
- Contextual help for common validation issues
- Prevention of invalid configuration downloads

### User Input Errors
- File path validation for different operating systems
- Regex pattern validation for log filters
- Numeric range validation for intervals
- Required field validation with clear messaging

### Runtime Errors
- Graceful handling of localStorage failures
- Network timeout handling for any future API calls
- JSON parsing error handling
- Browser compatibility fallbacks

## Testing Strategy

### Unit Testing
- Component testing with React Testing Library
- Configuration logic testing with Jest
- Validation function testing
- Template management testing
- OS-specific configuration testing

### Integration Testing
- End-to-end wizard flow testing
- Configuration generation and validation testing
- Template save/load functionality testing
- Cross-browser compatibility testing

### Accessibility Testing
- Keyboard navigation testing
- Screen reader compatibility testing
- Color contrast validation
- ARIA label verification
- Focus management testing

### Performance Testing
- Bundle size optimization
- Rendering performance with large configurations
- localStorage performance with many templates
- Memory usage monitoring

## Security Considerations

### Client-Side Security
- Input sanitization for all user inputs
- XSS prevention in configuration preview
- Safe regex pattern validation
- Secure template storage practices

### Data Privacy
- No sensitive data transmission (client-side only)
- Clear data retention policies for templates
- User control over data deletion
- No tracking or analytics without consent

## Deployment and Build

### Build Configuration
- Vite configuration for optimal bundling
- TypeScript strict mode enabled
- ESLint and Prettier for code quality
- Automated testing in CI/CD pipeline

### Static Hosting
- Deployable to any static hosting service
- CDN-friendly asset optimization
- Progressive Web App capabilities
- Offline functionality for core features

### Browser Support
- Modern browsers (Chrome 90+, Firefox 88+, Safari 14+, Edge 90+)
- Graceful degradation for older browsers
- Mobile-responsive design
- Touch-friendly interface elements