// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxprocessor

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	regexp           = "regexp"
	ActiveMqKey      = "activemq"
	CassandraKey     = "cassandra"
	HbaseKey         = "hbase"
	HadoopKey        = "hadoop"
	JettyKey         = "jetty"
	JvmKey           = "jvm"
	KafkaKey         = "kafka"
	KafkaConsumerKey = "kafka-consumer"
	KafkaProducerKey = "kafka-producer"
	SolrKey          = "solr"
	TomcatKey        = "tomcat"
	WildflyKey       = "wildfly"
)

var (
	jmxKey           = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey)
	activeMqKey      = common.ConfigKey(jmxKey, ActiveMqKey)
	cassandraKey     = common.ConfigKey(jmxKey, CassandraKey)
	hbaseKey         = common.ConfigKey(jmxKey, HbaseKey)
	hadoopKey        = common.ConfigKey(jmxKey, HadoopKey)
	jettyKey         = common.ConfigKey(jmxKey, JettyKey)
	jvmKey           = common.ConfigKey(jmxKey, JvmKey)
	kafkaKey         = common.ConfigKey(jmxKey, KafkaKey)
	kafkaConsumerKey = common.ConfigKey(jmxKey, KafkaConsumerKey)
	kafkaProducerKey = common.ConfigKey(jmxKey, KafkaProducerKey)
	solrKey          = common.ConfigKey(jmxKey, SolrKey)
	tomcatKey        = common.ConfigKey(jmxKey, TomcatKey)
	wildflyKey       = common.ConfigKey(jmxKey, WildflyKey)

	jmxTargets = []string{"activemq", "cassandra", "hbase", "hadoop", "jetty", "jvm", "kafka", "kafka-consumer", "kafka-producer", "solr", "tomcat", "wildfly"}
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, filterprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(jmxKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: fmt.Sprint(jmxKey)}
	}

	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)

	var includeMetricNames []string

	// When target name is set in configuration
	for _, jmxTarget := range jmxTargets {
		if conf.IsSet(jmxTarget) {
			includeMetricNames = append(includeMetricNames, t.getIncludeJmxMetrics(conf, jmxTarget)...)
		}
	}
	c := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{
			"include": map[string]interface{}{
				"match_type":   regexp,
				"metric_names": includeMetricNames,
			},
		},
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal jmx processor: %w", err)
	}

	return cfg, nil
}

func (t *translator) getIncludeJmxMetrics(conf *confmap.Conf, target string) []string {
	var includeMetricName []string
	targetMap := conf.Get(target)
	targetMetrics, ok := targetMap.(map[string]interface{})
	if !ok {
		// add regex to target when no metric names provided
		targetKeyRegex := target + ".*"
		includeMetricName = append(includeMetricName, targetKeyRegex)
	} else {
		for targetMetricName := range targetMetrics {
			includeMetricName = append(includeMetricName, targetMetricName)
		}
	}
	return includeMetricName
}

func IsSet(conf *confmap.Conf) bool {
	for _, jmxTarget := range jmxTargets {
		path := common.ConfigKey(jmxKey, jmxTarget)
		if conf.IsSet(path) {
			return true
		}
	}
	return false
}

