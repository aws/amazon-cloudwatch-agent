// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetrics

import (
	"bufio"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/systemmetrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

const batchTimeout = 15 * time.Minute

const (
	cgroupV2MemoryMaxPath   = "/sys/fs/cgroup/memory.max"
	cgroupV1MemoryLimitPath = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	apolloDir               = "/apollo"
	imageIDFile             = "/etc/image-id"
	osReleaseFile           = "/etc/os-release"
)

var amznMarkers = []string{"naws", "internal"}
var amznPrefixes = []string{"amzn2", "al2023"}

type translator struct{}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, common.PipelineNameSystemMetrics)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if !isEnabled(conf) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.SystemMetricsEnabledConfigKey}
	}

	isEC2 := !isOnPrem() && IsIMDSAvailable()

	processors := []common.ComponentTranslator{
		batchprocessor.NewTranslator(
			common.WithName(common.PipelineNameSystemMetrics),
			batchprocessor.WithTimeout(batchTimeout),
		),
	}
	if isEC2 {
		processors = append([]common.ComponentTranslator{newEc2TaggerTranslator()}, processors...)
	}

	translators := common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](systemmetrics.NewTranslator()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](processors...),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](newCloudWatchTranslator(isEC2)),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](
			agenthealth.NewTranslator(agenthealth.MetricsName, []string{agenthealth.OperationPutMetricData}),
		),
	}

	return &translators, nil
}

var IsIMDSAvailable = func() bool {
	return ec2util.GetEC2UtilSingleton().InstanceID != ""
}

var isOnPrem = func() bool {
	mode := context.CurrentContext().Mode()
	return mode == config.ModeOnPrem || mode == config.ModeOnPremise
}

func isKubernetes() bool {
	_, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	return ok
}

func isCgroupMemoryConstrained() bool {
	// cgroup v2: memory.max defaults to "max" (unlimited)
	if data, err := os.ReadFile(cgroupV2MemoryMaxPath); err == nil {
		if val := strings.TrimSpace(string(data)); val != "max" {
			return true
		}
	}
	// cgroup v1: memory.limit_in_bytes defaults to ~MaxInt64 when unlimited
	if data, err := os.ReadFile(cgroupV1MemoryLimitPath); err == nil {
		if limit, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			if limit < math.MaxInt64/2 {
				return true
			}
		}
	}
	return false
}

// isEnabled determines whether the systemmetrics pipeline should be created.
func isEnabled(conf *confmap.Conf) bool {
	if val, ok := os.LookupEnv(envconfig.SystemMetricsEnabled); ok {
		return strings.EqualFold(val, "true")
	}

	if conf != nil && conf.IsSet(common.SystemMetricsEnabledConfigKey) {
		enabled, ok := conf.Get(common.SystemMetricsEnabledConfigKey).(bool)
		if !ok {
			return false
		}
		return enabled
	}

	if context.CurrentContext().RunInContainer() || isKubernetes() || isCgroupMemoryConstrained() {
		return false
	}

	return isSystemMetricsHost()
}

// isSystemMetricsHost checks whether this host should collect system metrics.
func isSystemMetricsHost() bool {
	return hasApollo() || isRecognizedAMI()
}

func hasApollo() bool {
	info, err := os.Stat(apolloDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isRecognizedAMI() bool {
	return checkImageID() || checkOSRelease()
}

func checkImageID() bool {
	imageName := parseKeyFromFile(imageIDFile, "image_name")
	return matchesImageNameMarker(imageName)
}

func matchesImageNameMarker(imageName string) bool {
	lower := strings.ToLower(imageName)

	if strings.HasPrefix(lower, "al2-unified") {
		return true
	}
	hasPrefix := false
	for _, prefix := range amznPrefixes {
		if strings.HasPrefix(lower, prefix) {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		return false
	}
	for _, marker := range amznMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func checkOSRelease() bool {
	return parseKeyFromFile(osReleaseFile, "NAME") == "Amazon Linux" &&
		parseKeyFromFile(osReleaseFile, "VARIANT") == "internal"
}

// parseKeyFromFile reads a KEY=VALUE or KEY="VALUE" file and returns the value for the given key.
func parseKeyFromFile(path string, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	prefix := key + "="
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			val := line[len(prefix):]
			return strings.Trim(val, "\"")
		}
	}
	return ""
}
