import { useCallback } from 'react';
import { useAppContext } from '../context/AppContext';
import { ConfigTemplate, CloudWatchConfig, OperatingSystem } from '../types/config';

/**
 * Custom hook for template management
 */
export const useTemplates = () => {
  const { state, dispatch } = useAppContext();

  // Create a new template from current configuration
  const saveTemplate = useCallback((name: string, description?: string) => {
    if (!state.selectedOS) {
      throw new Error('Cannot save template without selected operating system');
    }

    const template: ConfigTemplate = {
      id: generateTemplateId(),
      name,
      description,
      createdAt: new Date(),
      updatedAt: new Date(),
      operatingSystem: state.selectedOS,
      configuration: { ...state.configuration }
    };

    dispatch({ type: 'SAVE_TEMPLATE', payload: template });
    return template;
  }, [state.selectedOS, state.configuration, dispatch]);

  // Update an existing template
  const updateTemplate = useCallback((templateId: string, updates: Partial<Pick<ConfigTemplate, 'name' | 'description'>>) => {
    const existingTemplate = state.templates.find(t => t.id === templateId);
    if (!existingTemplate) {
      throw new Error(`Template with id ${templateId} not found`);
    }

    const updatedTemplate: ConfigTemplate = {
      ...existingTemplate,
      ...updates,
      updatedAt: new Date()
    };

    dispatch({ type: 'SAVE_TEMPLATE', payload: updatedTemplate });
    return updatedTemplate;
  }, [state.templates, dispatch]);

  // Load a template into current configuration
  const loadTemplate = useCallback((templateId: string) => {
    const template = state.templates.find(t => t.id === templateId);
    if (!template) {
      throw new Error(`Template with id ${templateId} not found`);
    }

    dispatch({ type: 'LOAD_TEMPLATE', payload: templateId });
    return template;
  }, [state.templates, dispatch]);

  // Delete a template
  const deleteTemplate = useCallback((templateId: string) => {
    dispatch({ type: 'DELETE_TEMPLATE', payload: templateId });
  }, [dispatch]);

  // Get templates filtered by operating system
  const getTemplatesByOS = useCallback((os: OperatingSystem) => {
    return state.templates.filter(template => template.operatingSystem === os);
  }, [state.templates]);

  // Search templates by name or description
  const searchTemplates = useCallback((query: string) => {
    const lowercaseQuery = query.toLowerCase();
    return state.templates.filter(template => 
      template.name.toLowerCase().includes(lowercaseQuery) ||
      (template.description && template.description.toLowerCase().includes(lowercaseQuery))
    );
  }, [state.templates]);

  // Export template as JSON
  const exportTemplate = useCallback((templateId: string) => {
    const template = state.templates.find(t => t.id === templateId);
    if (!template) {
      throw new Error(`Template with id ${templateId} not found`);
    }

    const exportData = {
      ...template,
      exportedAt: new Date().toISOString(),
      version: '1.0'
    };

    return JSON.stringify(exportData, null, 2);
  }, [state.templates]);

  // Import template from JSON
  const importTemplate = useCallback((jsonData: string) => {
    try {
      const importedData = JSON.parse(jsonData);
      
      // Validate required fields
      if (!importedData.name || !importedData.operatingSystem || !importedData.configuration) {
        throw new Error('Invalid template format: missing required fields');
      }

      const template: ConfigTemplate = {
        id: generateTemplateId(), // Generate new ID to avoid conflicts
        name: importedData.name,
        description: importedData.description,
        createdAt: new Date(),
        updatedAt: new Date(),
        operatingSystem: importedData.operatingSystem,
        configuration: importedData.configuration
      };

      dispatch({ type: 'SAVE_TEMPLATE', payload: template });
      return template;
    } catch (error) {
      throw new Error(`Failed to import template: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }, [dispatch]);

  return {
    // State
    templates: state.templates,

    // Actions
    saveTemplate,
    updateTemplate,
    loadTemplate,
    deleteTemplate,
    getTemplatesByOS,
    searchTemplates,
    exportTemplate,
    importTemplate
  };
};

/**
 * Generate a unique template ID
 */
const generateTemplateId = (): string => {
  return `template_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
};