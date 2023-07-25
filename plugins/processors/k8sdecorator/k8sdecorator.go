// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/k8sdecorator/stores"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/k8sdecorator/structuredlogsadapter"
)

type K8sDecorator struct {
	started                 bool
	stores                  []stores.K8sStore
	shutdownC               chan bool
	DisableMetricExtraction bool   `toml:"disable_metric_extraction"`
	TagService              bool   `toml:"tag_service"`
	ClusterName             string `toml:"cluster_name"`
	HostIP                  string `toml:"host_ip"`
	NodeName                string `toml:"node_name"`
	PrefFullPodName         bool   `toml:"prefer_full_pod_name"`
}

func (k *K8sDecorator) Description() string {
	return ""
}

func (k *K8sDecorator) SampleConfig() string {
	return ""
}

func (k *K8sDecorator) Apply(in ...telegraf.Metric) []telegraf.Metric {
	if !k.started {
		k.start()
	}

	var out []telegraf.Metric

OUTER:
	for _, metric := range in {
		metric.AddTag(ClusterNameKey, k.ClusterName)
		k.handleHostname(metric)
		kubernetesBlob := make(map[string]interface{})
		for _, store := range k.stores {
			if !store.Decorate(metric, kubernetesBlob) {
				// drop the unexpected metric
				continue OUTER
			}
		}
		structuredlogsadapter.AddKubernetesInfo(metric, kubernetesBlob)
		structuredlogsadapter.TagMetricSource(metric)
		if !k.DisableMetricExtraction {
			structuredlogsadapter.TagMetricRule(metric)
		}
		structuredlogsadapter.TagLogGroup(metric)
		metric.AddTag(logscommon.LogStreamNameTag, k.NodeName)
		out = append(out, metric)
	}

	return out
}

// Shutdown currently does not get called, as telegraf does not have a cleanup hook for Filter plugins
func (k *K8sDecorator) Shutdown() {
	close(k.shutdownC)
}

func (k *K8sDecorator) start() {
	k.shutdownC = make(chan bool)

	k.stores = append(k.stores, stores.NewPodStore(k.HostIP, k.PrefFullPodName))
	if k.TagService {
		k.stores = append(k.stores, stores.NewServiceStore())
	}

	for _, store := range k.stores {
		store.RefreshTick()
	}

	go func() {
		refreshTicker := time.NewTicker(time.Second)
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				for _, store := range k.stores {
					store.RefreshTick()
				}
			case <-k.shutdownC:
				refreshTicker.Stop()
				return
			}
		}
	}()
	k.started = true
}

func (k *K8sDecorator) handleHostname(metric telegraf.Metric) {
	metricType := metric.Tags()[MetricType]
	// Add NodeName for node, pod and container
	if IsNode(metricType) || IsInstance(metricType) || IsPod(metricType) || IsContainer(metricType) {
		metric.AddTag(NodeNameKey, k.NodeName)
	}
	// remove the tag "host"
	metric.RemoveTag("host")
}

// init adds this plugin to the framework's "processors" registry
func init() {
	processors.Add("k8sdecorator", func() telegraf.Processor {
		return &K8sDecorator{TagService: true}
	})
}
