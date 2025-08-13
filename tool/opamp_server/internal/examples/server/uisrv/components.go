package uisrv

import (
    "github.com/open-telemetry/opamp-go/internal/examples/server/data"
    "gopkg.in/yaml.v3"
)

func extractComponents(agent *data.Agent) []map[string]interface{} {
    components := []map[string]interface{}{}
    
    // Parse the effective config
    var config map[string]interface{}
    if err := yaml.Unmarshal([]byte(agent.EffectiveConfig), &config); err != nil {
        return components
    }

    // Use a fixed version string since we can't access the private agentVersion field
    const version = "Todo: version not yet available"

    // Extract receivers
    if receivers, ok := config["receivers"].(map[string]interface{}); ok {
        for receiverName := range receivers {
            components = append(components, map[string]interface{}{
                "Type": "receivers",
                "Component": receiverName,
                "Version": version,
                "Used": true,
            })
        }
    }

    // Extract processors
    if processors, ok := config["processors"].(map[string]interface{}); ok {
        for processorName := range processors {
            components = append(components, map[string]interface{}{
                "Type": "processors",
                "Component": processorName,
                "Version": version,
                "Used": true,
            })
        }
    }

    // Extract exporters
    if exporters, ok := config["exporters"].(map[string]interface{}); ok {
        for exporterName := range exporters {
            components = append(components, map[string]interface{}{
                "Type": "exporters",
                "Component": exporterName,
                "Version": version,
                "Used": true,
            })
        }
    }

    // Extract extensions
    if extensions, ok := config["extensions"].(map[string]interface{}); ok {
        for extensionName := range extensions {
            components = append(components, map[string]interface{}{
                "Type": "extensions",
                "Component": extensionName,
                "Version": version,
                "Used": true,
            })
        }
    }
    
    return components
}

func extractPipelines(agent *data.Agent) map[string]interface{} {
    pipelines := make(map[string]interface{})
    
    // Parse the effective config
    var config map[string]interface{}
    if err := yaml.Unmarshal([]byte(agent.EffectiveConfig), &config); err != nil {
        return pipelines
    }

    service, ok := config["service"].(map[string]interface{})
    if !ok {
        return pipelines
    }

    servicePipelines, ok := service["pipelines"].(map[string]interface{})
    if !ok {
        return pipelines
    }

    // Extract each pipeline
    for name, pipeline := range servicePipelines {
        pipelineConfig, ok := pipeline.(map[string]interface{})
        if !ok {
            continue
        }

        pipelines[name] = map[string]interface{}{
            "healthy":    true,
            "receivers":  pipelineConfig["receivers"],
            "processors": pipelineConfig["processors"],
            "exporters":  pipelineConfig["exporters"],
        }
    }
    
    return pipelines
}