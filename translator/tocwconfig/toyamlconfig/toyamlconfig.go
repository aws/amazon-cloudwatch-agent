// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toyamlconfig

import (
	"bytes"
	"log"

	"gopkg.in/yaml.v3"
)

func ToYamlConfig(val interface{}) string {
	stripNullSendingQueueBatch(val)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		log.Panicf("Encode to a valid YAML config fails because of %v", err)
	}
	return buf.String()
}

// stripNullSendingQueueBatch removes a null `batch` key from any exporter's
// `sending_queue` before the config is serialized. When the translator emits an
// explicit sending_queue (to bound the profiles exporter's queue), the empty
// Batch optional marshals as `batch: null`. On reload the collector would
// promote that null to the otlphttp factory's default batch config and silently
// enable the exporter batcher, which the translator intends to keep off. No
// agent-emitted exporter configures a real (non-null) batch, so dropping only
// the null key keeps the batcher off and leaves every other exporter and any
// explicit batch untouched. Doing it here, the single YAML-emission point,
// keeps the generated config correct without a load-time converter and without
// ever rewriting customer-supplied OTEL YAML.
func stripNullSendingQueueBatch(val interface{}) {
	root, ok := val.(map[string]any)
	if !ok {
		return
	}
	exporters, ok := root["exporters"].(map[string]any)
	if !ok {
		return
	}
	for _, cfg := range exporters {
		exporterCfg, ok := cfg.(map[string]any)
		if !ok {
			continue
		}
		sendingQueue, ok := exporterCfg["sending_queue"].(map[string]any)
		if !ok {
			continue
		}
		if batch, present := sendingQueue["batch"]; present && batch == nil {
			delete(sendingQueue, "batch")
		}
	}
}
