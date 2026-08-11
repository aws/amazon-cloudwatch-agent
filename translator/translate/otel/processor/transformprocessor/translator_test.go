// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestContainerInsightsJmx(t *testing.T) {
	transl := NewTranslatorWithName(common.PipelineNameContainerInsightsJmx).(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_jmx_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)
	assert.Equal(t, len(expectedCfg.MetricStatements), len(actualCfg.MetricStatements))
}

func TestEfaTranslate(t *testing.T) {
	transl := NewTranslatorWithName(common.PipelineNameHostDeltaMetrics).(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_efa_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)
	assert.Equal(t, len(expectedCfg.MetricStatements), len(actualCfg.MetricStatements))
	assert.Equal(t, string(actualCfg.ErrorMode), "propagate")
	// Verify EFA attribute renaming statements are present
	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Contains(t, actualCfg.MetricStatements[0].Statements, `set(attributes["device"], attributes["aws.efa.device"]) where attributes["aws.efa.device"] != nil`)
	assert.Contains(t, actualCfg.MetricStatements[0].Statements, `set(attributes["port"], attributes["aws.efa.port"]) where attributes["aws.efa.port"] != nil`)
	assert.Contains(t, actualCfg.MetricStatements[0].Statements, `set(attributes["eniId"], attributes["aws.efa.eni.id"]) where attributes["aws.efa.eni.id"] != nil`)
}

func TestJmxTranslate(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_LINUX)
	transl := NewTranslatorWithName(common.PipelineNameJmx + "/drop").(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_jmx_drop_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)

	// sort the statements for consistency
	assert.Len(t, expectedCfg.MetricStatements, 1)
	assert.Len(t, actualCfg.MetricStatements, 1)
	sort.Strings(expectedCfg.MetricStatements[0].Statements)
	sort.Strings(actualCfg.MetricStatements[0].Statements)
	assert.Equal(t, string(actualCfg.ErrorMode), "propagate")
}

func TestDbiFixStartTimeTranslate(t *testing.T) {
	transl := NewTranslatorWithName(common.DbiTransformFixStartTime+"_"+common.PostgreSQLKey, WithDbiFixStartTime(common.PostgreSQLKey))
	assert.Equal(t, "transform/dbi_fix_start_time_postgresql", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)
	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "datapoint", string(actualCfg.MetricStatements[0].Context))
	require.Len(t, actualCfg.MetricStatements[0].Statements, 7)
	assert.Equal(t, "set(datapoint.start_time_unix_nano, datapoint.time_unix_nano) where datapoint.start_time_unix_nano == 0", actualCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `replace_match(datapoint.attributes["user.name"], "", "unknown")`, actualCfg.MetricStatements[0].Statements[6])
}

func TestDbiFixStartTimeMysqlTranslate(t *testing.T) {
	transl := NewTranslatorWithName(common.DbiTransformFixStartTime+"_"+common.MySQLKey, WithDbiFixStartTime(common.MySQLKey))
	assert.Equal(t, "transform/dbi_fix_start_time_mysql", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)
	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "datapoint", string(actualCfg.MetricStatements[0].Context))
	require.Len(t, actualCfg.MetricStatements[0].Statements, 5)
	assert.Equal(t, "set(datapoint.start_time_unix_nano, datapoint.time_unix_nano) where datapoint.start_time_unix_nano == 0", actualCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `replace_match(datapoint.attributes["mysql.wait_type"], "", "CPU")`, actualCfg.MetricStatements[0].Statements[1])
	assert.Equal(t, `replace_match(datapoint.attributes["user.name"], "", "unknown")`, actualCfg.MetricStatements[0].Statements[4])
}

func TestDbiFixStartTimeSqlServerTranslate(t *testing.T) {
	transl := NewTranslatorWithName(common.DbiTransformFixStartTime+"_"+common.SQLServerKey, WithDbiFixStartTime(common.SQLServerKey))
	assert.Equal(t, "transform/dbi_fix_start_time_sqlserver", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)
	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "datapoint", string(actualCfg.MetricStatements[0].Context))
	require.Len(t, actualCfg.MetricStatements[0].Statements, 6)
	assert.Equal(t, "set(datapoint.start_time_unix_nano, datapoint.time_unix_nano) where datapoint.start_time_unix_nano == 0", actualCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `replace_match(datapoint.attributes["sqlserver.wait_category"], "", "CPU")`, actualCfg.MetricStatements[0].Statements[1])
	assert.Equal(t, `replace_match(datapoint.attributes["user.name"], "", "unknown")`, actualCfg.MetricStatements[0].Statements[4])
	assert.Equal(t, `replace_match(datapoint.attributes["sqlserver.application.name"], "", "unknown")`, actualCfg.MetricStatements[0].Statements[5])
}

func TestDbiResourceTranslate(t *testing.T) {
	stmts := []string{
		`set(resource.attributes["db.system.name"], "postgresql")`,
		`set(resource.attributes["db.instance.name"], "my-db")`,
	}
	transl := NewTranslatorWithName(common.DbiTransformResource+"_0",
		WithMetricResourceStatements(stmts),
		WithLogResourceStatements(stmts),
	)
	assert.Equal(t, "transform/dbi_resource_0", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "resource", string(actualCfg.MetricStatements[0].Context))
	require.Len(t, actualCfg.MetricStatements[0].Statements, 2)
	assert.Equal(t, `set(resource.attributes["db.system.name"], "postgresql")`, actualCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `set(resource.attributes["db.instance.name"], "my-db")`, actualCfg.MetricStatements[0].Statements[1])

	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, actualCfg.MetricStatements[0].Statements, actualCfg.LogStatements[0].Statements)
}

func TestDbiLogDestinationTranslate(t *testing.T) {
	stmts := []string{
		`set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/postgresql/raw-events")`,
		`set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "my-db"], "/"))`,
	}
	transl := NewTranslatorWithName(common.DbiTransformLogs+"_raw-events_0", WithLogResourceStatements(stmts))
	assert.Equal(t, "transform/dbi_logs_raw-events_0", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, "resource", string(actualCfg.LogStatements[0].Context))
	assert.Equal(t, "ignore", string(actualCfg.LogStatements[0].ErrorMode))
	require.Len(t, actualCfg.LogStatements[0].Statements, 2)
	assert.Equal(t, `set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/postgresql/raw-events")`, actualCfg.LogStatements[0].Statements[0])
	assert.Equal(t, `set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "my-db"], "/"))`, actualCfg.LogStatements[0].Statements[1])
	assert.Empty(t, actualCfg.MetricStatements)
}

func TestLogScopeStatements(t *testing.T) {
	transl := NewTranslatorWithName("test_log_scope",
		WithLogScopeStatements(common.ScopeStatementsForSolution("otel-test")),
	)
	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, "scope", string(actualCfg.LogStatements[0].Context))
	assert.Equal(t, "ignore", string(actualCfg.LogStatements[0].ErrorMode))
	require.Len(t, actualCfg.LogStatements[0].Statements, 2)
	assert.Equal(t, `set(scope.attributes["cloudwatch.source"], "cloudwatch-agent")`, actualCfg.LogStatements[0].Statements[0])
	assert.Equal(t, `set(scope.attributes["cloudwatch.solution"], "otel-test")`, actualCfg.LogStatements[0].Statements[1])
	assert.Empty(t, actualCfg.MetricStatements)
	assert.Empty(t, actualCfg.TraceStatements)
}

func TestLogContextStatements(t *testing.T) {
	transl := NewTranslatorWithName("test_log_context",
		WithLogContextStatements([]string{
			`delete_key(attributes, "timestamp")`,
			`delete_key(attributes, "log.file.name")`,
		}),
	)
	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, "log", string(actualCfg.LogStatements[0].Context))
	assert.Equal(t, "ignore", string(actualCfg.LogStatements[0].ErrorMode))
	require.Len(t, actualCfg.LogStatements[0].Statements, 2)
	assert.Equal(t, `delete_key(attributes, "timestamp")`, actualCfg.LogStatements[0].Statements[0])
	assert.Equal(t, `delete_key(attributes, "log.file.name")`, actualCfg.LogStatements[0].Statements[1])
	assert.Empty(t, actualCfg.MetricStatements)
	assert.Empty(t, actualCfg.TraceStatements)
}

func TestLogScopeAndLogContextStatements(t *testing.T) {
	transl := NewTranslatorWithName("test_combined",
		WithLogScopeStatements(common.ScopeStatementsForSolution("otel-test")),
		WithLogContextStatements([]string{`delete_key(attributes, "timestamp")`}),
	)
	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.LogStatements, 2)
	assert.Equal(t, "scope", string(actualCfg.LogStatements[0].Context))
	assert.Equal(t, "log", string(actualCfg.LogStatements[1].Context))
	require.Len(t, actualCfg.LogStatements[1].Statements, 1)
	assert.Equal(t, `delete_key(attributes, "timestamp")`, actualCfg.LogStatements[1].Statements[0])
}

func TestMetricScopeStatements(t *testing.T) {
	transl := NewTranslatorWithName("test_metric_scope",
		WithMetricScopeStatements(common.ScopeStatementsForSolution("otel-test")),
	)
	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "scope", string(actualCfg.MetricStatements[0].Context))
	assert.Equal(t, "ignore", string(actualCfg.MetricStatements[0].ErrorMode))
	require.Len(t, actualCfg.MetricStatements[0].Statements, 2)
	assert.Equal(t, `set(scope.attributes["cloudwatch.source"], "cloudwatch-agent")`, actualCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `set(scope.attributes["cloudwatch.solution"], "otel-test")`, actualCfg.MetricStatements[0].Statements[1])
	assert.Empty(t, actualCfg.LogStatements)
	assert.Empty(t, actualCfg.TraceStatements)
}

func TestWithErrorMode(t *testing.T) {
	transl := NewTranslatorWithName("test_error_mode",
		WithErrorMode("propagate"),
		WithLogResourceStatements([]string{`set(resource.attributes["key"], "val")`}),
	)
	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	assert.Equal(t, "propagate", string(actualCfg.ErrorMode))
	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, "propagate", string(actualCfg.LogStatements[0].ErrorMode))
}

func TestScopeStatementsAllSignals(t *testing.T) {
	transl := NewTranslatorWithName("test_all_scope",
		WithScopeStatements(common.ScopeStatementsForSolution("otel-test")),
	)
	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "scope", string(actualCfg.MetricStatements[0].Context))
	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, "scope", string(actualCfg.LogStatements[0].Context))
	require.Len(t, actualCfg.TraceStatements, 1)
	assert.Equal(t, "scope", string(actualCfg.TraceStatements[0].Context))
}

func TestLogsRoutingWindowsSync(t *testing.T) {
	type routingConfig struct {
		LogStatements []struct {
			Statements []string `yaml:"statements"`
		} `yaml:"log_statements"`
	}

	baseBytes, err := os.ReadFile("transform_logs_routing_host.yaml")
	require.NoError(t, err)
	winBytes, err := os.ReadFile("transform_logs_routing_host_windows.yaml")
	require.NoError(t, err)

	var base, win routingConfig
	require.NoError(t, yaml.Unmarshal(baseBytes, &base))
	require.NoError(t, yaml.Unmarshal(winBytes, &win))

	require.Len(t, base.LogStatements, 1)
	require.Len(t, win.LogStatements, 1)

	baseStmts := base.LogStatements[0].Statements
	winStmts := win.LogStatements[0].Statements

	// Partition Windows statements into channel-routing and shared
	var channelStmts, sharedStmts []string
	for _, stmt := range winStmts {
		if strings.Contains(stmt, "aws.log.channel") {
			channelStmts = append(channelStmts, stmt)
		} else {
			sharedStmts = append(sharedStmts, stmt)
		}
	}

	assert.Equal(t, 2, len(channelStmts),
		"Windows routing YAML must have exactly 2 channel-routing statements")
	assert.Equal(t, baseStmts, sharedStmts,
		"Windows routing YAML shared statements must match base")

	// Channel routing must be gated on the source the agent itself sets, or an
	// OTLP client sending aws.log.channel could dictate its own stream name.
	guard := fmt.Sprintf(`resource.attributes["aws.log.source"] == %q`, common.WindowsEventsKey)
	for _, stmt := range channelStmts {
		assert.Contains(t, stmt, guard,
			"channel-routing statement must be guarded by aws.log.source")
	}
}

// TestLogsRoutingSourceGuards asserts that every stream-name rule keying off an
// attribute the agent itself attaches per source is gated on aws.log.source.
// Without the guard an OTLP client can set that attribute and choose its own log
// stream. Covers both source-specific attributes in both host routing YAMLs.
func TestLogsRoutingSourceGuards(t *testing.T) {
	type routingConfig struct {
		LogStatements []struct {
			Statements []string `yaml:"statements"`
		} `yaml:"log_statements"`
	}

	// attribute -> the aws.log.source value that must gate rules using it
	sourceAttrs := map[string]string{
		"log.file.name":   common.FilesKey,
		"aws.log.channel": common.WindowsEventsKey,
	}

	for _, path := range []string{"transform_logs_routing_host.yaml", "transform_logs_routing_host_windows.yaml"} {
		t.Run(path, func(t *testing.T) {
			b, err := os.ReadFile(path)
			require.NoError(t, err)
			var cfg routingConfig
			require.NoError(t, yaml.Unmarshal(b, &cfg))
			require.Len(t, cfg.LogStatements, 1)

			var guarded int
			for _, stmt := range cfg.LogStatements[0].Statements {
				if !strings.Contains(stmt, `set(resource.attributes["aws.log.stream.name"]`) {
					continue
				}
				for attr, source := range sourceAttrs {
					if !strings.Contains(stmt, fmt.Sprintf(`resource.attributes[%q]`, attr)) {
						continue
					}
					assert.Contains(t, stmt, fmt.Sprintf(`resource.attributes["aws.log.source"] == %q`, source),
						"stream rule using %q must be guarded by aws.log.source == %q", attr, source)
					guarded++
				}
			}
			// Guard against the assertions passing vacuously if the rules are renamed.
			assert.NotZero(t, guarded, "expected at least one source-specific stream rule")
		})
	}
}

// TestIdentityTransformSemconvValues asserts that the semconv constants used by
// resource detection detectors match the values our identity transform OTTL
// rules depend on. If a dependency bump changes these, the OTTL must be updated.
func TestIdentityTransformSemconvValues(t *testing.T) {
	// cloud.platform values used in OTTL WHERE clauses
	assert.Equal(t, "aws_ec2", semconv.AttributeCloudPlatformAWSEC2)
	assert.Equal(t, "aws_ecs", semconv.AttributeCloudPlatformAWSECS)
	assert.Equal(t, "aws_eks", semconv.AttributeCloudPlatformAWSEKS)
	assert.Equal(t, "azure_vm", semconv.AttributeCloudPlatformAzureVM)
	assert.Equal(t, "azure_aks", semconv.AttributeCloudPlatformAzureAKS)

	// Resource attribute keys used in OTTL statements
	assert.Equal(t, "cloud.account.id", semconv.AttributeCloudAccountID)
	assert.Equal(t, "cloud.region", semconv.AttributeCloudRegion)
	assert.Equal(t, "cloud.platform", semconv.AttributeCloudPlatform)
	assert.Equal(t, "host.id", semconv.AttributeHostID)
	assert.Equal(t, "host.name", semconv.AttributeHostName)
	assert.Equal(t, "k8s.cluster.name", semconv.AttributeK8SClusterName)
	assert.Equal(t, "k8s.namespace.name", semconv.AttributeK8SNamespaceName)
	assert.Equal(t, "k8s.deployment.name", semconv.AttributeK8SDeploymentName)
	assert.Equal(t, "k8s.pod.name", semconv.AttributeK8SPodName)
	assert.Equal(t, "k8s.container.name", semconv.AttributeK8SContainerName)
	assert.Equal(t, "service.name", semconv.AttributeServiceName)
	assert.Equal(t, "service.namespace", semconv.AttributeServiceNamespace)
	assert.Equal(t, "service.instance.id", semconv.AttributeServiceInstanceID)
	assert.Equal(t, "service.version", semconv.AttributeServiceVersion)
}

// TestAKSClusterResourceIDDerivation guards the AKS cloud.resource_id fix: the k8s identity transform
// must derive the cluster's resource group from the node's MC_<clusterRG>_<cluster>_<region> RG rather
// than using azure.resourcegroup.name (the node RG) directly, and the Azure VM transform must NOT --
// there the detected RG is the correct VM identity.
func TestAKSClusterResourceIDDerivation(t *testing.T) {
	// The derivation lives only in the k8s transform.
	assert.Contains(t, transformIdentityK8sConfig, `replace_pattern(resource.attributes["_tmp.azure.resourcegroup.name"]`,
		"k8s identity transform must derive the cluster RG via replace_pattern")
	assert.Contains(t, transformIdentityK8sConfig, `delete_key(resource.attributes, "_tmp.azure.resourcegroup.name")`,
		"k8s identity transform must clean up the temp attribute")
	assert.NotContains(t, transformIdentityAzureVMConfig, "_tmp.azure.resourcegroup.name",
		"Azure VM transform must use the detected RG directly, not derive it")

	// Every %s in the AKS Format is nil-guarded (a nil arg would render a corrupt %!s(<nil>) ID).
	for _, attr := range []string{"cloud.account.id", "_tmp.azure.resourcegroup.name", "k8s.cluster.name"} {
		assert.Contains(t, transformIdentityK8sConfig,
			fmt.Sprintf(`resource.attributes[%q] != nil`, attr),
			"AKS resource_id statement must guard %q", attr)
	}
}

// TestResolveK8sIdentityConfig asserts the cluster name is injected into the regex literal at
// translate time (replace_pattern's regex is compile-time, so it cannot read the attribute at runtime).
func TestResolveK8sIdentityConfig(t *testing.T) {
	// No cluster name configured -> underscore-free fallback segment.
	got := injectClusterName(confmap.New())
	assert.NotContains(t, got, "%CLUSTER_NAME%", "placeholder must be substituted")
	assert.Contains(t, got, `"^MC_(.+)_[^_]+_[^_]+$"`, "empty cluster name should fall back to [^_]+")

	// Configured cluster name -> injected verbatim into the regex literal. ValidateClusterName
	// (enforced on every path reaching this transform) restricts names to [A-Za-z0-9-_], none of
	// which are regex metacharacters, so no escaping is needed.
	conf := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{"cluster_name": "my_cluster"},
	})
	got = injectClusterName(conf)
	assert.NotContains(t, got, "%CLUSTER_NAME%")
	assert.Contains(t, got, `"^MC_(.+)_my_cluster_[^_]+$"`, "cluster name must be injected into the regex literal")
}
