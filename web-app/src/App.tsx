import React from 'react';
import { AppProvider } from './context/AppContext';
import { useConfiguration } from './hooks/useConfiguration';
import { useTemplates } from './hooks/useTemplates';

const ConfigurationDemo: React.FC = () => {
  const {
    selectedOS,
    currentStep,
    configuration,
    isValid,
    setOperatingSystem,
    nextStep,
    previousStep,
    updateMetrics,
    validateConfiguration
  } = useConfiguration();

  const {
    templates,
    saveTemplate
  } = useTemplates();

  const handleSaveTemplate = () => {
    if (selectedOS) {
      try {
        saveTemplate('Demo Template', 'A demo template for testing');
        alert('Template saved successfully!');
      } catch (error) {
        alert(`Error saving template: ${error instanceof Error ? error.message : 'Unknown error'}`);
      }
    }
  };

  return (
    <div style={{ padding: '20px', fontFamily: 'Arial, sans-serif' }}>
      <h1>CloudWatch Config Generator - State Management Demo</h1>
      
      <div style={{ marginBottom: '20px' }}>
        <h2>Current State</h2>
        <p><strong>Step:</strong> {currentStep}</p>
        <p><strong>Selected OS:</strong> {selectedOS || 'None'}</p>
        <p><strong>Is Valid:</strong> {isValid ? 'Yes' : 'No'}</p>
        <p><strong>Templates Count:</strong> {templates.length}</p>
      </div>

      <div style={{ marginBottom: '20px' }}>
        <h2>OS Selection</h2>
        <button onClick={() => setOperatingSystem('linux')}>Select Linux</button>
        <button onClick={() => setOperatingSystem('windows')} style={{ marginLeft: '10px' }}>Select Windows</button>
        <button onClick={() => setOperatingSystem('darwin')} style={{ marginLeft: '10px' }}>Select macOS</button>
      </div>

      <div style={{ marginBottom: '20px' }}>
        <h2>Step Navigation</h2>
        <button onClick={previousStep}>Previous Step</button>
        <button onClick={nextStep} style={{ marginLeft: '10px' }}>Next Step</button>
      </div>

      <div style={{ marginBottom: '20px' }}>
        <h2>Configuration</h2>
        <button onClick={() => updateMetrics({ namespace: 'CustomNamespace' })}>
          Update Metrics Namespace
        </button>
        <button onClick={validateConfiguration} style={{ marginLeft: '10px' }}>
          Validate Configuration
        </button>
      </div>

      <div style={{ marginBottom: '20px' }}>
        <h2>Templates</h2>
        <button onClick={handleSaveTemplate}>Save Current as Template</button>
      </div>

      <div>
        <h2>Configuration Preview</h2>
        <pre style={{ background: '#f5f5f5', padding: '10px', overflow: 'auto' }}>
          {JSON.stringify(configuration, null, 2)}
        </pre>
      </div>
    </div>
  );
};

const App: React.FC = () => {
  return (
    <AppProvider>
      <ConfigurationDemo />
    </AppProvider>
  );
};

export default App;