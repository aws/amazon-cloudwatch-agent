import { AppState, AppAction } from '../types/state';
import { CloudWatchConfig } from '../types/config';

export const initialState: AppState = {
  currentStep: 0,
  selectedOS: null,
  configuration: {
    agent: {
      metrics_collection_interval: 60,
      debug: false
    },
    metrics: {
      namespace: 'CWAgent',
      metrics_collected: {}
    },
    logs: {
      logs_collected: {
        files: {
          collect_list: []
        }
      }
    }
  },
  validationErrors: [],
  templates: [],
  isValid: false
};

export const appReducer = (state: AppState, action: AppAction): AppState => {
  switch (action.type) {
    case 'SET_OS':
      return {
        ...state,
        selectedOS: action.payload,
        // Reset configuration when OS changes to ensure compatibility
        configuration: {
          ...initialState.configuration,
          agent: state.configuration.agent
        }
      };

    case 'SET_STEP':
      return {
        ...state,
        currentStep: action.payload
      };

    case 'UPDATE_METRICS':
      return {
        ...state,
        configuration: {
          ...state.configuration,
          metrics: {
            ...state.configuration.metrics,
            ...action.payload
          }
        }
      };

    case 'UPDATE_LOGS':
      return {
        ...state,
        configuration: {
          ...state.configuration,
          logs: {
            ...state.configuration.logs,
            ...action.payload
          }
        }
      };

    case 'ADD_LOG_FILE':
      const currentFiles = state.configuration.logs?.logs_collected?.files?.collect_list || [];
      return {
        ...state,
        configuration: {
          ...state.configuration,
          logs: {
            ...state.configuration.logs,
            logs_collected: {
              ...state.configuration.logs?.logs_collected,
              files: {
                collect_list: [...currentFiles, action.payload]
              }
            }
          }
        }
      };

    case 'REMOVE_LOG_FILE':
      const filesAfterRemoval = state.configuration.logs?.logs_collected?.files?.collect_list?.filter(
        (_, index) => index !== action.payload
      ) || [];
      return {
        ...state,
        configuration: {
          ...state.configuration,
          logs: {
            ...state.configuration.logs,
            logs_collected: {
              ...state.configuration.logs?.logs_collected,
              files: {
                collect_list: filesAfterRemoval
              }
            }
          }
        }
      };

    case 'UPDATE_LOG_FILE':
      const updatedFiles = state.configuration.logs?.logs_collected?.files?.collect_list?.map(
        (file, index) => index === action.payload.index ? action.payload.logFile : file
      ) || [];
      return {
        ...state,
        configuration: {
          ...state.configuration,
          logs: {
            ...state.configuration.logs,
            logs_collected: {
              ...state.configuration.logs?.logs_collected,
              files: {
                collect_list: updatedFiles
              }
            }
          }
        }
      };

    case 'SET_VALIDATION_ERRORS':
      return {
        ...state,
        validationErrors: action.payload,
        isValid: action.payload.length === 0
      };

    case 'VALIDATE_CONFIG':
      // This will be implemented when validation logic is added
      // For now, just mark as valid if OS is selected
      const isValid = state.selectedOS !== null;
      return {
        ...state,
        isValid,
        validationErrors: isValid ? [] : [{ field: 'os', message: 'Operating system must be selected' }]
      };

    case 'SAVE_TEMPLATE':
      const existingTemplateIndex = state.templates.findIndex(t => t.id === action.payload.id);
      let updatedTemplates;
      
      if (existingTemplateIndex >= 0) {
        // Update existing template
        updatedTemplates = state.templates.map((template, index) =>
          index === existingTemplateIndex ? action.payload : template
        );
      } else {
        // Add new template
        updatedTemplates = [...state.templates, action.payload];
      }

      return {
        ...state,
        templates: updatedTemplates
      };

    case 'LOAD_TEMPLATE':
      const templateToLoad = state.templates.find(t => t.id === action.payload);
      if (!templateToLoad) {
        return state;
      }

      return {
        ...state,
        selectedOS: templateToLoad.operatingSystem,
        configuration: { ...templateToLoad.configuration },
        currentStep: 0
      };

    case 'DELETE_TEMPLATE':
      return {
        ...state,
        templates: state.templates.filter(t => t.id !== action.payload)
      };

    case 'LOAD_TEMPLATES':
      return {
        ...state,
        templates: action.payload
      };

    case 'RESET_CONFIG':
      return {
        ...initialState,
        templates: state.templates // Keep templates when resetting
      };

    default:
      return state;
  }
};