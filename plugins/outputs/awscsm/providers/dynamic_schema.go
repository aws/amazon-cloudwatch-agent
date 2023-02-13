// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"fmt"
	"sort"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/csm"
)

// Definitions ...
type Definitions struct {
	Entries EventEntryDefinitions
	Events  EventDefinitions
}

func (d *Definitions) add(output *csm.GetPublishingSchemaOutput) {
	d.Entries.add(output)
	d.Events.add(output)
}

func (d *Definitions) clear() {
	d.Entries.clear()
	d.Events.clear()
}

func (d Definitions) String() string {
	return fmt.Sprintf("\tEvents:\n%s\n\tEntries:\n%s\n", d.Events.String(), d.Entries.String())
}

// EventEntryDefinition represents a definition of an event and
// the necessary information on how this metric should be
// aggregated.
type EventEntryDefinition struct {
	KeyType EventEntryKeyType
	Type    MetricType
	Name    string
}

func (d *EventEntryDefinitions) add(output *csm.GetPublishingSchemaOutput) {
	for _, schema := range output.MonitoringEventEntrySchemas {
		if schema.Name == nil {
			continue
		}

		// No schema was defined other than the name
		if schema.KeyType == nil && schema.MetricType == nil {
			continue
		}

		keyType := EventEntryKeyType(csm.MonitoringEventEntryKeyTypeNone)
		var err error

		if schema.KeyType != nil {
			keyType, err = NewEventEntryKeyType(schema.KeyType)
			if cerr, ok := err.(ContinueError); ok && cerr.Continue() {
				continue
			} else if err != nil {
				break
			}
		}

		t := MetricType(csm.MonitoringEventEntryMetricTypeNone)
		if schema.MetricType != nil {
			t, err = NewMetricType(schema.MetricType)
			if cerr, ok := err.(ContinueError); ok && cerr.Continue() {
				continue
			} else if err != nil {
				break
			}
		}

		def := EventEntryDefinition{
			KeyType: keyType,
			Type:    t,
			Name:    *schema.Name,
		}

		d.Set(def.Name, def)
	}
}

// EventEntryDefinitions is a map of metric definitions
type EventEntryDefinitions struct {
	container map[string]EventEntryDefinition
}

// Set will set a metric definition to the given key
func (d *EventEntryDefinitions) Set(key string, def EventEntryDefinition) {
	d.container[key] = def
}

// Get ...
func (d *EventEntryDefinitions) Get(key string) (EventEntryDefinition, bool) {
	v, ok := d.container[key]
	return v, ok
}

func (d *EventEntryDefinitions) clear() {
	d.container = map[string]EventEntryDefinition{}
}

func (d EventEntryDefinitions) String() string {
	if d.container == nil {
		return "\t\tnil"
	}

	if len(d.container) == 0 {
		return "\t\tempty"
	}
	str := ""
	keys := []string{}
	for k := range d.container {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		str += "\n\t\t" + k
	}
	return str + "\n"
}

// EventDefinitions ...
type EventDefinitions struct {
	container map[string]EventDefinition
}

func (d EventDefinitions) String() string {
	if d.container == nil {
		return "\t\tnil"
	}

	if len(d.container) == 0 {
		return "\t\tempty"
	}
	str := ""
	keys := []string{}
	for k := range d.container {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		str += "\n\t\t" + k
	}
	return str + "\n"
}

func (d *EventDefinitions) add(output *csm.GetPublishingSchemaOutput) {
	for _, schema := range output.MonitoringEventSchemas {
		key := ""
		sampleLimit := int64(0)
		sampleRate := float64(0.0)

		if schema.Name != nil {
			key = *schema.Name
		}
		if schema.MaxSampleCount != nil {
			sampleLimit = *schema.MaxSampleCount
		}
		if schema.SampleRate != nil {
			sampleRate = *schema.SampleRate
		}

		def := EventDefinition{
			Name:           key,
			MaxSampleCount: sampleLimit,
			SampleRate:     sampleRate,
		}
		d.Set(key, def)
	}
}

// Set will set a metric definition to the given key
func (d *EventDefinitions) Set(key string, def EventDefinition) {
	d.container[key] = def
}

// Get will retrieve the event definition and if one was not present at
// the given key, it will return false for the second parameter.
func (d *EventDefinitions) Get(key string) (EventDefinition, bool) {
	v, ok := d.container[key]
	return v, ok
}

func (d *EventDefinitions) clear() {
	d.container = map[string]EventDefinition{}
}

// EventDefinition represents a definition of a monitoring event.
type EventDefinition struct {
	Name           string
	SampleRate     float64
	MaxSampleCount int64
}

// DefaultEventEntryDefinitions ...
func DefaultEventEntryDefinitions() EventEntryDefinitions {
	return EventEntryDefinitions{
		container: map[string]EventEntryDefinition{
			// key type schemas
			apiDefinition.Name:       apiDefinition,
			clientIDDefinition.Name:  clientIDDefinition,
			serviceDefinition.Name:   serviceDefinition,
			timestampDefinition.Name: timestampDefinition,
			typeDefinition.Name:      typeDefinition,
			regionDefinition.Name:    regionDefinition,
			userAgentDefinition.Name: userAgentDefinition,
			// frequency schemas
			attemptCountDefinition.Name:             attemptCountDefinition,
			maxRetriesExceededDefinition.Name:       maxRetriesExceededDefinition,
			apiCallTimeoutDefinition.Name:           apiCallTimeoutDefinition,
			httpStatusCodeDefinition.Name:           httpStatusCodeDefinition,
			finalHttpStatusCodeDefinition.Name:      finalHttpStatusCodeDefinition,
			sdkExceptionDefinition.Name:             sdkExceptionDefinition,
			finalSdkExceptionDefinition.Name:        finalSdkExceptionDefinition,
			sdkExceptionMessageDefinition.Name:      sdkExceptionMessageDefinition,
			finalSdkExceptionMessageDefinition.Name: finalSdkExceptionMessageDefinition,
			awsExceptionDefinition.Name:             awsExceptionDefinition,
			finalAwsExceptionDefinition.Name:        finalAwsExceptionDefinition,
			awsExceptionMessageDefinition.Name:      awsExceptionMessageDefinition,
			finalAwsExceptionMessageDefinition.Name: finalAwsExceptionMessageDefinition,
			destinationIPDefinition.Name:            destinationIPDefinition,
			connectionReusedDefinition.Name:         connectionReusedDefinition,
			// seh schemas
			acquireConnectionLatencyDefinition.Name: acquireConnectionLatencyDefinition,
			attemptLatencyDefinition.Name:           attemptLatencyDefinition,
			connectLatencyDefinition.Name:           connectLatencyDefinition,
			dnsLatencyDefinition.Name:               dnsLatencyDefinition,
			latencyDefinition.Name:                  latencyDefinition,
			requestLatencyDefinition.Name:           requestLatencyDefinition,
			sslLatencyDefinition.Name:               sslLatencyDefinition,
			tcpLatencyDefinition.Name:               tcpLatencyDefinition,
			// sample only schemas
			fqdnDefinition.Name:                fqdnDefinition,
			sessionTokenDefinition.Name:        sessionTokenDefinition,
			akidDefinition.Name:                akidDefinition,
			awsExceptionMessageDefinition.Name: awsExceptionMessageDefinition,
			sdkExceptionMessageDefinition.Name: sdkExceptionMessageDefinition,
		},
	}
}

// DefaultEventDefinitions will return a specified default of EventDefinitions
// which will be used before the schema had been retrieved by the provider.
func DefaultEventDefinitions() EventDefinitions {
	return EventDefinitions{
		container: map[string]EventDefinition{
			"ApiCall": {
				Name:           "ApiCall",
				SampleRate:     1.0,
				MaxSampleCount: 1,
			},
			"ApiCallAttempt": {
				Name:           "ApiCallAttempt",
				SampleRate:     1.0,
				MaxSampleCount: 10,
			},
		},
	}
}

// DefaultDefinitions will return the default definitions used by the agent
// prior to reaching out to the control plane.
func DefaultDefinitions() Definitions {
	return Definitions{
		Entries: DefaultEventEntryDefinitions(),
		Events:  DefaultEventDefinitions(),
	}
}
