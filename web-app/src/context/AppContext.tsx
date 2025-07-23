import React, { createContext, useContext, useReducer, useEffect, ReactNode } from 'react';
import { AppState, AppAction } from '../types/state';
import { CloudWatchConfig, ConfigTemplate } from '../types/config';
import { appReducer, initialState } from './appReducer';
import { saveToLocalStorage, loadFromLocalStorage } from '../utils/localStorage';

interface AppContextType {
  state: AppState;
  dispatch: React.Dispatch<AppAction>;
}

const AppContext = createContext<AppContextType | undefined>(undefined);

interface AppProviderProps {
  children: ReactNode;
}

export const AppProvider: React.FC<AppProviderProps> = ({ children }) => {
  const [state, dispatch] = useReducer(appReducer, initialState);

  // Load templates from localStorage on mount
  useEffect(() => {
    const savedTemplates = loadFromLocalStorage<ConfigTemplate[]>('cloudwatch-templates', []);
    dispatch({ type: 'LOAD_TEMPLATES', payload: savedTemplates });
  }, []);

  // Save templates to localStorage whenever templates change
  useEffect(() => {
    saveToLocalStorage('cloudwatch-templates', state.templates);
  }, [state.templates]);

  // Save current configuration to localStorage whenever it changes
  useEffect(() => {
    const configToSave = {
      selectedOS: state.selectedOS,
      configuration: state.configuration,
      currentStep: state.currentStep
    };
    saveToLocalStorage('cloudwatch-current-config', configToSave);
  }, [state.selectedOS, state.configuration, state.currentStep]);

  // Load current configuration from localStorage on mount
  useEffect(() => {
    const savedConfig = loadFromLocalStorage<{
      selectedOS: AppState['selectedOS'];
      configuration: CloudWatchConfig;
      currentStep: number;
    }>('cloudwatch-current-config', null);

    if (savedConfig) {
      if (savedConfig.selectedOS) {
        dispatch({ type: 'SET_OS', payload: savedConfig.selectedOS });
      }
      if (savedConfig.configuration) {
        if (savedConfig.configuration.metrics) {
          dispatch({ type: 'UPDATE_METRICS', payload: savedConfig.configuration.metrics });
        }
        if (savedConfig.configuration.logs) {
          dispatch({ type: 'UPDATE_LOGS', payload: savedConfig.configuration.logs });
        }
      }
      if (savedConfig.currentStep) {
        dispatch({ type: 'SET_STEP', payload: savedConfig.currentStep });
      }
    }
  }, []);

  return (
    <AppContext.Provider value={{ state, dispatch }}>
      {children}
    </AppContext.Provider>
  );
};

export const useAppContext = (): AppContextType => {
  const context = useContext(AppContext);
  if (context === undefined) {
    throw new Error('useAppContext must be used within an AppProvider');
  }
  return context;
};