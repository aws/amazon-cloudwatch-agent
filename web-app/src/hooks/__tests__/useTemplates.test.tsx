import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { ReactNode } from 'react';
import { AppProvider } from '../../context/AppContext';
import { useTemplates } from '../useTemplates';
import { useConfiguration } from '../useConfiguration';
import { ConfigTemplate } from '../../types/config';

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

describe('useTemplates', () => {
  it('should provide initial empty templates', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    expect(result.current.templates).toEqual([]);
  });

  it('should save a template', () => {
    const { result: configResult } = renderHook(() => useConfiguration(), { wrapper });
    const { result: templatesResult } = renderHook(() => useTemplates(), { wrapper });
    
    // Set up some configuration
    act(() => {
      configResult.current.setOperatingSystem('linux');
      configResult.current.updateMetrics({
        namespace: 'TestNamespace'
      });
    });
    
    // Save template
    let savedTemplate: ConfigTemplate;
    act(() => {
      savedTemplate = templatesResult.current.saveTemplate('Test Template', 'A test template');
    });
    
    expect(templatesResult.current.templates).toHaveLength(1);
    expect(templatesResult.current.templates[0].name).toBe('Test Template');
    expect(templatesResult.current.templates[0].description).toBe('A test template');
    expect(templatesResult.current.templates[0].operatingSystem).toBe('linux');
    expect(templatesResult.current.templates[0].configuration.metrics?.namespace).toBe('TestNamespace');
  });

  it('should throw error when saving template without OS', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    expect(() => {
      act(() => {
        result.current.saveTemplate('Test Template');
      });
    }).toThrow('Cannot save template without selected operating system');
  });

  it('should update an existing template', () => {
    const { result: configResult } = renderHook(() => useConfiguration(), { wrapper });
    const { result: templatesResult } = renderHook(() => useTemplates(), { wrapper });
    
    // Set up configuration and save template
    act(() => {
      configResult.current.setOperatingSystem('linux');
    });
    
    let templateId: string;
    act(() => {
      const template = templatesResult.current.saveTemplate('Original Name', 'Original description');
      templateId = template.id;
    });
    
    // Update template
    act(() => {
      templatesResult.current.updateTemplate(templateId, {
        name: 'Updated Name',
        description: 'Updated description'
      });
    });
    
    expect(templatesResult.current.templates).toHaveLength(1);
    expect(templatesResult.current.templates[0].name).toBe('Updated Name');
    expect(templatesResult.current.templates[0].description).toBe('Updated description');
  });

  it('should throw error when updating non-existent template', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    expect(() => {
      act(() => {
        result.current.updateTemplate('non-existent-id', { name: 'New Name' });
      });
    }).toThrow('Template with id non-existent-id not found');
  });

  it('should load a template', () => {
    const { result: configResult } = renderHook(() => useConfiguration(), { wrapper });
    const { result: templatesResult } = renderHook(() => useTemplates(), { wrapper });
    
    // Set up configuration and save template
    act(() => {
      configResult.current.setOperatingSystem('windows');
      configResult.current.updateMetrics({
        namespace: 'WindowsNamespace'
      });
    });
    
    let templateId: string;
    act(() => {
      const template = templatesResult.current.saveTemplate('Windows Template');
      templateId = template.id;
    });
    
    // Change configuration
    act(() => {
      configResult.current.setOperatingSystem('linux');
      configResult.current.updateMetrics({
        namespace: 'LinuxNamespace'
      });
    });
    
    // Load template
    act(() => {
      templatesResult.current.loadTemplate(templateId);
    });
    
    expect(configResult.current.selectedOS).toBe('windows');
    expect(configResult.current.configuration.metrics?.namespace).toBe('WindowsNamespace');
    expect(configResult.current.currentStep).toBe(0);
  });

  it('should throw error when loading non-existent template', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    expect(() => {
      act(() => {
        result.current.loadTemplate('non-existent-id');
      });
    }).toThrow('Template with id non-existent-id not found');
  });

  it('should delete a template', () => {
    const { result: configResult } = renderHook(() => useConfiguration(), { wrapper });
    const { result: templatesResult } = renderHook(() => useTemplates(), { wrapper });
    
    // Save two templates
    act(() => {
      configResult.current.setOperatingSystem('linux');
    });
    
    let templateId1: string, templateId2: string;
    act(() => {
      const template1 = templatesResult.current.saveTemplate('Template 1');
      const template2 = templatesResult.current.saveTemplate('Template 2');
      templateId1 = template1.id;
      templateId2 = template2.id;
    });
    
    expect(templatesResult.current.templates).toHaveLength(2);
    
    // Delete first template
    act(() => {
      templatesResult.current.deleteTemplate(templateId1);
    });
    
    expect(templatesResult.current.templates).toHaveLength(1);
    expect(templatesResult.current.templates[0].name).toBe('Template 2');
  });

  it('should filter templates by OS', () => {
    const { result: configResult } = renderHook(() => useConfiguration(), { wrapper });
    const { result: templatesResult } = renderHook(() => useTemplates(), { wrapper });
    
    // Save templates for different OS
    act(() => {
      configResult.current.setOperatingSystem('linux');
      templatesResult.current.saveTemplate('Linux Template');
      
      configResult.current.setOperatingSystem('windows');
      templatesResult.current.saveTemplate('Windows Template');
      
      configResult.current.setOperatingSystem('linux');
      templatesResult.current.saveTemplate('Another Linux Template');
    });
    
    const linuxTemplates = templatesResult.current.getTemplatesByOS('linux');
    const windowsTemplates = templatesResult.current.getTemplatesByOS('windows');
    
    expect(linuxTemplates).toHaveLength(2);
    expect(windowsTemplates).toHaveLength(1);
    expect(linuxTemplates[0].name).toBe('Linux Template');
    expect(linuxTemplates[1].name).toBe('Another Linux Template');
    expect(windowsTemplates[0].name).toBe('Windows Template');
  });

  it('should search templates', () => {
    const { result: configResult } = renderHook(() => useConfiguration(), { wrapper });
    const { result: templatesResult } = renderHook(() => useTemplates(), { wrapper });
    
    // Save templates with different names and descriptions
    act(() => {
      configResult.current.setOperatingSystem('linux');
      templatesResult.current.saveTemplate('Production Config', 'Configuration for production environment');
      templatesResult.current.saveTemplate('Development Setup', 'Development environment configuration');
      templatesResult.current.saveTemplate('Test Environment', 'Testing configuration');
    });
    
    const productionResults = templatesResult.current.searchTemplates('production');
    const devResults = templatesResult.current.searchTemplates('dev');
    const configResults = templatesResult.current.searchTemplates('config');
    
    expect(productionResults).toHaveLength(1);
    expect(productionResults[0].name).toBe('Production Config');
    
    expect(devResults).toHaveLength(1);
    expect(devResults[0].name).toBe('Development Setup');
    
    expect(configResults).toHaveLength(2); // Matches both name and description
  });

  it('should export template as JSON', () => {
    const { result: configResult } = renderHook(() => useConfiguration(), { wrapper });
    const { result: templatesResult } = renderHook(() => useTemplates(), { wrapper });
    
    act(() => {
      configResult.current.setOperatingSystem('linux');
    });
    
    let templateId: string;
    act(() => {
      const template = templatesResult.current.saveTemplate('Export Test');
      templateId = template.id;
    });
    
    const exportedJson = templatesResult.current.exportTemplate(templateId);
    const exportedData = JSON.parse(exportedJson);
    
    expect(exportedData.name).toBe('Export Test');
    expect(exportedData.operatingSystem).toBe('linux');
    expect(exportedData.exportedAt).toBeDefined();
    expect(exportedData.version).toBe('1.0');
  });

  it('should throw error when exporting non-existent template', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    expect(() => {
      result.current.exportTemplate('non-existent-id');
    }).toThrow('Template with id non-existent-id not found');
  });

  it('should import template from JSON', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    const templateData = {
      name: 'Imported Template',
      description: 'An imported template',
      operatingSystem: 'windows',
      configuration: {
        agent: { debug: true },
        metrics: { namespace: 'ImportedNamespace', metrics_collected: {} }
      }
    };
    
    const jsonData = JSON.stringify(templateData);
    
    let importedTemplate: ConfigTemplate;
    act(() => {
      importedTemplate = result.current.importTemplate(jsonData);
    });
    
    expect(result.current.templates).toHaveLength(1);
    expect(result.current.templates[0].name).toBe('Imported Template');
    expect(result.current.templates[0].operatingSystem).toBe('windows');
    expect(result.current.templates[0].configuration.agent?.debug).toBe(true);
  });

  it('should throw error when importing invalid JSON', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    expect(() => {
      act(() => {
        result.current.importTemplate('invalid json');
      });
    }).toThrow('Failed to import template');
  });

  it('should throw error when importing template with missing fields', () => {
    const { result } = renderHook(() => useTemplates(), { wrapper });
    
    const incompleteData = {
      name: 'Incomplete Template'
      // Missing operatingSystem and configuration
    };
    
    expect(() => {
      act(() => {
        result.current.importTemplate(JSON.stringify(incompleteData));
      });
    }).toThrow('Invalid template format: missing required fields');
  });
});