// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/hash"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/filelog"
)

type fileEntry struct {
	index            int
	filePath         string
	encoding         string
	multilinePattern string
	timestampFormat  string
	timezone         string
	logGroupName     string
	logStreamName    string
	resource         map[string]string
}

func (e fileEntry) name() string {
	return fmt.Sprintf("%s_%d", common.SanitizeName(e.filePath), e.index)
}

func (e fileEntry) receiverName() string {
	return fmt.Sprintf("%s_%s", common.SanitizeName(e.filePath), e.receiverHash())
}

func (e fileEntry) receiverHash() string {
	return hash.HashName(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s",
		e.filePath, e.encoding, e.multilinePattern, e.timestampFormat, e.timezone))
}

func (e fileEntry) routingAttributes() map[string]string {
	if e.logGroupName == "" && e.logStreamName == "" {
		return nil
	}
	attrs := make(map[string]string)
	if e.logGroupName != "" {
		attrs["aws.log.group.name"] = e.logGroupName
	}
	if e.logStreamName != "" {
		attrs["aws.log.stream.name"] = e.logStreamName
	}
	return attrs
}

type filesPipelineTranslator struct {
	entry fileEntry
}

var _ common.PipelineTranslator = (*filesPipelineTranslator)(nil)

func (t *filesPipelineTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, "files_"+t.entry.name())
}

func (t *filesPipelineTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	receiverOpts := []filelog.Option{
		filelog.WithName(t.entry.receiverName()),
		filelog.WithFilePath(t.entry.filePath),
		filelog.WithEncoding(t.entry.encoding),
		filelog.WithResource(t.entry.resource),
		filelog.WithStorage(),
		filelog.WithStartAtBeginning(),
	}
	if t.entry.multilinePattern != "" {
		receiverOpts = append(receiverOpts, filelog.WithMultilinePattern(t.entry.multilinePattern))
	}
	if t.entry.timestampFormat != "" {
		receiverOpts = append(receiverOpts, filelog.WithTimestampFormat(t.entry.timestampFormat, t.entry.timezone))
	}

	receivers := common.NewTranslatorMap[component.Config, component.ID]()
	receivers.Set(filelog.NewTranslator(receiverOpts...))

	processors := common.NewTranslatorMap[component.Config, component.ID]()
	if attrs := t.entry.routingAttributes(); len(attrs) > 0 {
		processors.Set(resourceprocessor.NewTranslator(
			common.WithName("files_"+t.entry.name()),
			resourceprocessor.WithAttributes(attrs),
		))
	}
	processors.Set(transformprocessor.NewTranslatorWithName("files_scope",
		transformprocessor.WithErrorMode(common.OTTLErrorModeIgnore),
		transformprocessor.WithLogScopeStatements(common.ScopeStatementsForSolution("otel-files")),
		transformprocessor.WithLogLogStatements([]string{
			`set(resource.attributes["aws.log.file.name"], attributes["log.file.name"]) where attributes["log.file.name"] != nil`,
			`delete_key(attributes, "log.file.name")`,
			`delete_key(attributes, "timestamp")`,
		}),
	))

	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](filestorage.NewTranslator()),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
