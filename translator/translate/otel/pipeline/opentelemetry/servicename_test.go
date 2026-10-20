// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapstructure"
	"github.com/aws/amazon-cloudwatch-agent/service/configprovider"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toyamlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	otelotlp "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/opentelemetry/otlp"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const serviceNameAttribute = "service.name"

var baseTranslators = map[pipeline.Signal]common.PipelineTranslator{
	pipeline.SignalMetrics: NewBaseMetricsTranslator(),
	pipeline.SignalLogs:    NewBaseLogsTranslator(),
	pipeline.SignalTraces:  NewBaseTracesTranslator(),
}

var otlpConf = map[string]interface{}{
	"opentelemetry": map[string]interface{}{
		"collect": map[string]interface{}{
			"otlp": map[string]interface{}{},
		},
	},
}

type serviceNamePlatform struct {
	mode           string
	k8sMode        string
	ecs            bool
	ec2ServiceName string
}

func (p serviceNamePlatform) apply(t *testing.T) {
	t.Helper()
	translatorcontext.ResetContext()
	t.Cleanup(translatorcontext.ResetContext)
	mode := p.mode
	if mode == "" {
		mode = config.ModeEC2
	}
	translatorcontext.CurrentContext().SetMode(mode)
	translatorcontext.CurrentContext().SetKubernetesMode(p.k8sMode)
	t.Setenv("K8S_NODE_NAME", "node_name_from_env")
	previousRegion := ecsutil.GetECSUtilSingleton().Region
	ecsutil.GetECSUtilSingleton().Region = ""
	if p.ecs {
		ecsutil.GetECSUtilSingleton().Region = "us-east-1"
	}
	t.Cleanup(func() { ecsutil.GetECSUtilSingleton().Region = previousRegion })
	previousProvider := otelotlp.EC2ServiceNameProvider
	otelotlp.EC2ServiceNameProvider = func() string { return p.ec2ServiceName }
	t.Cleanup(func() { otelotlp.EC2ServiceNameProvider = previousProvider })
	previousConfig := agent.Global_Config
	agent.Global_Config = agent.Agent{Region: "us-east-1"}
	t.Cleanup(func() { agent.Global_Config = previousConfig })
}

func TestServiceNameBehavior(t *testing.T) {
	testCases := map[string]struct {
		serviceNamePlatform
		attributes map[string]string
		want       string
	}{
		"EC2/Absent":                           {want: "unknown_service"},
		"EC2/InferredFromHost":                 {serviceNamePlatform: serviceNamePlatform{ec2ServiceName: "web-tier"}, want: "web-tier"},
		"EC2/SenderBeatsInferred":              {serviceNamePlatform: serviceNamePlatform{ec2ServiceName: "web-tier"}, attributes: map[string]string{serviceNameAttribute: "payments"}, want: "payments"},
		"EC2/SDKUnknownServiceBeatsInferred":   {serviceNamePlatform: serviceNamePlatform{ec2ServiceName: "web-tier"}, attributes: map[string]string{serviceNameAttribute: "unknown_service:java"}, want: "unknown_service:java"},
		"EC2/SDKUnknownServicePassesThrough":   {attributes: map[string]string{serviceNameAttribute: "unknown_service:java"}, want: "unknown_service:java"},
		"ECS/TaskFamilyNamesService":           {serviceNamePlatform: serviceNamePlatform{ecs: true, ec2ServiceName: "not-used-on-ecs"}, attributes: map[string]string{"aws.ecs.task.family": "payments-task"}, want: "payments-task"},
		"ECS/SenderBeatsTaskFamily":            {serviceNamePlatform: serviceNamePlatform{ecs: true}, attributes: map[string]string{serviceNameAttribute: "payments", "aws.ecs.task.family": "payments-task"}, want: "payments"},
		"ECS/SDKUnknownServiceBeatsTaskFamily": {serviceNamePlatform: serviceNamePlatform{ecs: true}, attributes: map[string]string{serviceNameAttribute: "unknown_service:java", "aws.ecs.task.family": "payments-task"}, want: "unknown_service:java"},
		"ECS/Absent":                           {serviceNamePlatform: serviceNamePlatform{ecs: true}, want: "unknown_service"},
		"OnPrem/Absent":                        {serviceNamePlatform: serviceNamePlatform{mode: config.ModeOnPrem, ec2ServiceName: "not-used-off-ec2"}, want: "unknown_service"},
		"AzureVM/Absent":                       {serviceNamePlatform: serviceNamePlatform{mode: config.ModeAzureVM, ec2ServiceName: "not-used-off-ec2"}, want: "unknown_service"},
		"GCE/SenderStamped":                    {serviceNamePlatform: serviceNamePlatform{mode: config.ModeGCE}, attributes: map[string]string{serviceNameAttribute: "payments"}, want: "payments"},
	}
	for name, testCase := range testCases {
		for signal, translator := range baseTranslators {
			t.Run(signal.String()+"/"+name, func(t *testing.T) {
				testCase.apply(t)
				conf := confmap.NewFromStringMap(otlpConf)
				got, err := translator.Translate(conf)
				require.NoError(t, err)
				attributes := runServiceNameProcessors(t, signal, got, conf, testCase.attributes)
				value, ok := attributes.Get(serviceNameAttribute)
				require.True(t, ok)
				assert.Equal(t, testCase.want, value.Str())
			})
		}
	}
}

func TestServiceNameInferenceOwnership(t *testing.T) {
	testCases := map[string]struct {
		serviceNamePlatform
		wantInferred bool
	}{
		"EC2":            {serviceNamePlatform: serviceNamePlatform{ec2ServiceName: "web-tier"}, wantInferred: true},
		"EC2/NoHostName": {},
		"ECS":            {serviceNamePlatform: serviceNamePlatform{ecs: true, ec2ServiceName: "not-used-on-ecs"}, wantInferred: true},
		"EKS":            {serviceNamePlatform: serviceNamePlatform{k8sMode: config.ModeEKS, ec2ServiceName: "not-used-on-kubernetes"}},
		"OnPrem":         {serviceNamePlatform: serviceNamePlatform{mode: config.ModeOnPrem, ec2ServiceName: "not-used-off-ec2"}},
		"AzureVM":        {serviceNamePlatform: serviceNamePlatform{mode: config.ModeAzureVM}},
		"GCE":            {serviceNamePlatform: serviceNamePlatform{mode: config.ModeGCE}},
	}
	for name, testCase := range testCases {
		for signal, translator := range baseTranslators {
			t.Run(signal.String()+"/"+name, func(t *testing.T) {
				testCase.apply(t)
				conf := confmap.NewFromStringMap(otlpConf)
				got, err := translator.Translate(conf)
				require.NoError(t, err)
				keys := got.Processors.Keys()
				inferred := component.MustNewIDWithName("resource", signal.String())
				identity := component.MustNewIDWithName("transform", common.Identity)
				inferredIndex, identityIndex := -1, -1
				for i, id := range keys {
					switch id {
					case inferred:
						inferredIndex = i
					case identity:
						identityIndex = i
					}
				}
				require.GreaterOrEqual(t, identityIndex, 0)
				if testCase.wantInferred {
					assert.Equal(t, identityIndex-1, inferredIndex)
				} else {
					assert.Equal(t, -1, inferredIndex)
				}
				for _, id := range keys {
					processorTranslator, ok := got.Processors.Get(id)
					require.True(t, ok)
					cfg, err := processorTranslator.Translate(conf)
					require.NoError(t, err)
					switch c := cfg.(type) {
					case *transformprocessor.Config:
						for _, statement := range transformStatements(c) {
							if !strings.HasPrefix(statement, `set(resource.attributes["service.name"]`) {
								assert.NotContains(t, statement, `resource.attributes["service.name"],`, id.String())
								continue
							}
							assert.Equal(t, identity, id)
							assert.Contains(t, statement, `where resource.attributes["service.name"] == nil`)
						}
					case *resourceprocessor.Config:
						for _, action := range c.AttributesActions {
							if id != inferred {
								assert.NotEqual(t, serviceNameAttribute, action.Key, id.String())
								continue
							}
							assert.Equal(t, serviceNameAttribute, action.Key)
							assert.EqualValues(t, "insert", action.Action)
							assert.NotEqual(t, "unknown_service", action.Value)
						}
					}
				}
			})
		}
	}
}

func transformStatements(cfg *transformprocessor.Config) []string {
	var statements []string
	for _, cs := range cfg.MetricStatements {
		statements = append(statements, cs.Statements...)
	}
	for _, cs := range cfg.LogStatements {
		statements = append(statements, cs.Statements...)
	}
	for _, cs := range cfg.TraceStatements {
		statements = append(statements, cs.Statements...)
	}
	return statements
}

func loadProcessorsAsAgent(t *testing.T, translators *common.ComponentTranslators, conf *confmap.Conf) *confmap.Conf {
	t.Helper()
	processors := map[string]any{}
	translators.Processors.Range(func(translator common.ComponentTranslator) {
		cfg, err := translator.Translate(conf)
		require.NoError(t, err)
		processors[translator.ID().String()] = cfg
	})
	marshalled, err := mapstructure.Marshal(map[string]any{"processors": processors})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(toyamlconfig.ToYamlConfig(marshalled)), 0600))

	resolver, err := confmap.NewResolver(configprovider.GetSettings([]string{"file:" + path}, zap.NewNop()).ResolverSettings)
	require.NoError(t, err)
	loaded, err := resolver.Resolve(context.Background())
	require.NoError(t, err)
	return loaded
}

// runServiceNameProcessors runs the translated service.name chain, resource/<signal> (when
// present) then transform/identity, in pipeline order through the contrib factories.
func runServiceNameProcessors(t *testing.T, signal pipeline.Signal, translators *common.ComponentTranslators, conf *confmap.Conf, attributes map[string]string) pcommon.Map {
	t.Helper()
	loaded := loadProcessorsAsAgent(t, translators, conf)
	_, hasInferred := translators.Processors.Get(component.MustNewIDWithName("resource", signal.String()))
	chain := func(sink any) component.Component {
		first := newProcessor(t, signal, transformprocessor.NewFactory(), loaded, "transform", common.Identity, sink)
		if hasInferred {
			first = newProcessor(t, signal, resourceprocessor.NewFactory(), loaded, "resource", signal.String(), first)
		}
		return first
	}
	ctx := context.Background()
	switch signal {
	case pipeline.SignalMetrics:
		sink := new(consumertest.MetricsSink)
		metrics := pmetric.NewMetrics()
		fill(metrics.ResourceMetrics().AppendEmpty().Resource().Attributes(), attributes)
		require.NoError(t, chain(sink).(consumer.Metrics).ConsumeMetrics(ctx, metrics))
		require.Len(t, sink.AllMetrics(), 1)
		return sink.AllMetrics()[0].ResourceMetrics().At(0).Resource().Attributes()
	case pipeline.SignalLogs:
		sink := new(consumertest.LogsSink)
		logs := plog.NewLogs()
		fill(logs.ResourceLogs().AppendEmpty().Resource().Attributes(), attributes)
		require.NoError(t, chain(sink).(consumer.Logs).ConsumeLogs(ctx, logs))
		require.Len(t, sink.AllLogs(), 1)
		return sink.AllLogs()[0].ResourceLogs().At(0).Resource().Attributes()
	default:
		sink := new(consumertest.TracesSink)
		traces := ptrace.NewTraces()
		fill(traces.ResourceSpans().AppendEmpty().Resource().Attributes(), attributes)
		require.NoError(t, chain(sink).(consumer.Traces).ConsumeTraces(ctx, traces))
		require.Len(t, sink.AllTraces(), 1)
		return sink.AllTraces()[0].ResourceSpans().At(0).Resource().Attributes()
	}
}

func fill(target pcommon.Map, attributes map[string]string) {
	for key, value := range attributes {
		target.PutStr(key, value)
	}
}

func newProcessor(t *testing.T, signal pipeline.Signal, factory processor.Factory, loaded *confmap.Conf, typ, name string, next any) component.Component {
	t.Helper()
	cfg := factory.CreateDefaultConfig()
	sub, err := loaded.Sub("processors::" + component.MustNewIDWithName(typ, name).String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	settings := processortest.NewNopSettings(factory.Type())
	var proc component.Component
	switch signal {
	case pipeline.SignalMetrics:
		proc, err = factory.CreateMetrics(context.Background(), settings, cfg, next.(consumer.Metrics))
	case pipeline.SignalLogs:
		proc, err = factory.CreateLogs(context.Background(), settings, cfg, next.(consumer.Logs))
	default:
		proc, err = factory.CreateTraces(context.Background(), settings, cfg, next.(consumer.Traces))
	}
	require.NoError(t, err)
	require.NoError(t, proc.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, proc.Shutdown(context.Background())) })
	return proc
}
