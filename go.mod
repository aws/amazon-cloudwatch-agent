module github.com/aws/amazon-cloudwatch-agent

go 1.24.4

replace github.com/influxdata/telegraf => github.com/aws/telegraf v0.10.2-0.20250113150713-a2dfaa4cdf6d

// Replace with https://github.com/amazon-contributing/opentelemetry-collector-contrib, there are no requirements for all receivers/processors/exporters
// to be all replaced since there are some changes that will always be from upstream
replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter => github.com/amazon-contributing/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter => github.com/amazon-contributing/opentelemetry-collector-contrib/exporter/awsemfexporter v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter => github.com/amazon-contributing/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.0.0-20250717174233-f514045fc484
)

replace (
	github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware => github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy => github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsproxy v0.0.0-20250717174233-f514045fc484
)

replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/awsutil v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/containerinsight v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/cwlogs v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/k8s v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/metrics v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/proxy v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/xray v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/k8sconfig v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/kubelet v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/metadataproviders v0.0.0-20250717174233-f514045fc484
)

replace (
	// For clear resource attributes after copy functionality https://github.com/amazon-contributing/opentelemetry-collector-contrib/pull/148
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => github.com/amazon-contributing/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => github.com/amazon-contributing/opentelemetry-collector-contrib/pkg/stanza v0.0.0-20250717174233-f514045fc484
	// Replace with contrib to revert upstream change https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/20519
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => github.com/amazon-contributing/opentelemetry-collector-contrib/pkg/translator/prometheus v0.0.0-20250717174233-f514045fc484
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => github.com/amazon-contributing/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.0.0-20250717174233-f514045fc484

replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/jmxreceiver v0.0.0-20250717174233-f514045fc484
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.0.0-20250717174233-f514045fc484
)

// Temporary fix, pending PR https://github.com/shirou/gopsutil/pull/957
replace github.com/shirou/gopsutil/v3 => github.com/aws/telegraf/patches/gopsutil/v3 v3.0.0-20250113150713-a2dfaa4cdf6d // indirect

//pin consul to a newer version to fix the ambiguous import issue
//see https://github.com/hashicorp/consul/issues/6019 and https://github.com/hashicorp/consul/issues/6414
//Consul is used only by an plugin in telegraf and the version changes from v1.2.1 to v1.8.4 here (no major version change)
//so the replacement should not affect amazon-cloudwatch-agent
replace github.com/hashicorp/consul => github.com/hashicorp/consul v1.8.4

//consul@v1.8.4 points to a commit in go-discover that depends on an older version of kubernetes: kubernetes-1.16.9
//https://github.com/hashicorp/consul/blob/12b16df320052414244659e4dadda078f67849ed/go.mod#L38
//This commit contains a dependency in launchpad.net which requires the version control system Bazaar to be set up
//https://github.com/hashicorp/go-discover/commit/ad1e96bde088162a25dc224d687440181b704162#diff-37aff102a57d3d7b797f152915a6dc16R44
//To avoid the requirement for Bazaar, we want to replace go-discover with a newer version. However to avoid the upgrade of k8s.io lib from
//0.17.4 (used by current amazon-cloudwatch-agent) to 0.18.x, we choose the commit just before go-discover introduced k8s.io 0.18.8
//go-discover is used only by consul and consul is used only in telegraf, so the replacement here should not affect amazon-cloudwatch-agent
replace github.com/hashicorp/go-discover => github.com/hashicorp/go-discover v0.0.0-20200713171816-3392d2f47463

//proxy.golang.org has versions of golang.zx2c4.com/wireguard with leading v's, whereas the git repo has tags without
//leading v's: https://git.zx2c4.com/wireguard-go/refs/tags
//So, fetching this module with version v0.0.20200121 (as done by the transitive dependency
//https://github.com/WireGuard/wgctrl-go/blob/e35592f146e40ce8057113d14aafcc3da231fbac/go.mod#L12 ) was not working when
//using GOPROXY=direct.
//Replacing with the pseudo-version works around this.
replace golang.zx2c4.com/wireguard v0.0.20200121 => golang.zx2c4.com/wireguard v0.0.0-20200121152719-05b03c675090

// BurntSushi 0.4.1 do not decode .toml with '[]' into empty slice anymore which breaks confmigrate.
replace github.com/BurntSushi/toml v0.4.1 => github.com/BurntSushi/toml v0.3.1

// To prevent empty slices from overwriting OTel defaults such as telemetry/logs/output_paths (change in behaviour with v1.5.1)
replace github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4 => github.com/mitchellh/mapstructure v1.5.0

replace github.com/karrick/godirwalk v1.16.1 => github.com/karrick/godirwalk v1.12.0

replace github.com/docker/distribution => github.com/docker/distribution v2.8.2+incompatible

// go-kit has the fix for nats-io/jwt/v2 merged but not released yet. Replacing this version for now until next release.
replace github.com/go-kit/kit => github.com/go-kit/kit v0.12.1-0.20220808180842-62c81a0f3047

// openshift removed all tags from their repo, use the pseudoversion from the release-3.9 branch HEAD
replace github.com/openshift/api v3.9.0+incompatible => github.com/openshift/api v0.0.0-20180801171038-322a19404e37

// forces version bump to support log group classes
replace github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.48.6

require (
	github.com/BurntSushi/toml v1.3.2
	github.com/Jeffail/gabs v1.4.0
	github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware v0.124.0
	github.com/aws/aws-sdk-go v1.55.7
	github.com/aws/aws-sdk-go-v2 v1.36.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.30.2
	github.com/bigkevmcd/go-configparser v0.0.0-20200217161103-d137835d2579
	github.com/deckarep/golang-set/v2 v2.3.1
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gin-gonic/gin v1.10.0
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-playground/validator/v10 v10.20.0
	github.com/go-test/deep v1.0.2-0.20181118220953-042da051cf31
	github.com/gobwas/glob v0.2.3
	github.com/google/btree v1.1.3
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	github.com/hashicorp/golang-lru v1.0.2
	github.com/influxdata/telegraf v0.0.0-00010101000000-000000000000
	github.com/influxdata/wlog v0.0.0-20160411224016-7c63b0a71ef8
	github.com/jellydator/ttlcache/v3 v3.3.0
	github.com/json-iterator/go v1.1.12
	github.com/kardianos/service v1.2.1 // Keep this pinned to v1.2.1. v1.2.2 causes the agent to not register as a service on Windows
	github.com/knadh/koanf v1.5.0
	github.com/knadh/koanf/v2 v2.2.0
	github.com/kr/pretty v0.3.1
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c
	github.com/oklog/run v1.1.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor v0.124.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver v0.0.0-20250613205757-3ab9974bf1ed
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.124.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.22.0
	github.com/prometheus/common v0.62.0
	github.com/prometheus/prometheus v0.300.1
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/stretchr/testify v1.10.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.opentelemetry.io/collector/client v1.30.0
	go.opentelemetry.io/collector/component v1.30.0
	go.opentelemetry.io/collector/component/componenttest v0.124.0
	go.opentelemetry.io/collector/config/configauth v0.124.0
	go.opentelemetry.io/collector/config/confighttp v0.124.0
	go.opentelemetry.io/collector/config/configopaque v1.30.0
	go.opentelemetry.io/collector/config/configtelemetry v0.124.0
	go.opentelemetry.io/collector/config/configtls v1.30.0
	go.opentelemetry.io/collector/confmap v1.30.0
	go.opentelemetry.io/collector/confmap/converter/expandconverter v0.113.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.30.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.30.0
	go.opentelemetry.io/collector/confmap/xconfmap v0.124.0
	go.opentelemetry.io/collector/consumer v1.30.0
	go.opentelemetry.io/collector/consumer/consumertest v0.124.0
	go.opentelemetry.io/collector/exporter v0.124.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.124.0
	go.opentelemetry.io/collector/exporter/exportertest v0.124.0
	go.opentelemetry.io/collector/exporter/nopexporter v0.124.0
	go.opentelemetry.io/collector/extension v1.30.0
	go.opentelemetry.io/collector/extension/extensiontest v0.124.0
	go.opentelemetry.io/collector/extension/zpagesextension v0.124.0
	go.opentelemetry.io/collector/filter v0.124.0
	go.opentelemetry.io/collector/otelcol v0.124.0
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.124.0
	go.opentelemetry.io/collector/pdata v1.30.0
	go.opentelemetry.io/collector/pipeline v0.124.0
	go.opentelemetry.io/collector/processor v1.30.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.124.0
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.124.0
	go.opentelemetry.io/collector/processor/processorhelper v0.124.0
	go.opentelemetry.io/collector/processor/processortest v0.124.0
	go.opentelemetry.io/collector/receiver v1.30.0
	go.opentelemetry.io/collector/receiver/nopreceiver v0.124.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.124.0
	go.opentelemetry.io/collector/receiver/receivertest v0.124.0
	go.opentelemetry.io/collector/scraper v0.124.0
	go.opentelemetry.io/collector/scraper/scraperhelper v0.124.0
	go.opentelemetry.io/collector/semconv v0.124.0
	go.opentelemetry.io/collector/service v0.124.0
	go.uber.org/atomic v1.11.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8
	golang.org/x/net v0.39.0
	golang.org/x/sync v0.13.0
	golang.org/x/sys v0.32.0
	golang.org/x/text v0.24.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools/v3 v3.2.0
	k8s.io/api v0.32.3
	k8s.io/apimachinery v0.32.3
	k8s.io/client-go v0.32.3
	k8s.io/klog/v2 v2.130.1
)

require (
	cloud.google.com/go/auth v0.15.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	collectd.org v0.4.0 // indirect
	github.com/Azure/azure-sdk-for-go v67.1.0+incompatible // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.17.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.8.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4 v4.3.0 // indirect
	github.com/Azure/go-autorest/autorest v0.11.29 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.23 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.3.3 // indirect
	github.com/Code-Hex/go-generics-cache v1.5.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.27.0 // indirect
	github.com/IBM/sarama v1.45.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.12.9 // indirect
	github.com/Shopify/sarama v1.37.2 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/alecthomas/participle v0.4.1 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws v0.0.0-20250519142056-97a2f5e08ffa // indirect
	github.com/antchfx/jsonquery v1.1.5 // indirect
	github.com/antchfx/xmlquery v1.4.4 // indirect
	github.com/antchfx/xpath v1.3.4 // indirect
	github.com/apache/arrow/go/v12 v12.0.1 // indirect
	github.com/apache/arrow/go/v15 v15.0.2 // indirect
	github.com/apache/thrift v0.21.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-msk-iam-sasl-signer-go v1.0.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.14 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.67 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.211.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.19 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/bboreham/go-loser v0.0.0-20230920113527-fcc2c21820a3 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.8.1 // indirect
	github.com/bytedance/sonic v1.11.6 // indirect
	github.com/bytedance/sonic/loader v0.1.1 // indirect
	github.com/caio/go-tdigest v3.1.0+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/cncf/xds/go v0.0.0-20250121191232-2f005788dc42 // indirect
	github.com/containerd/cgroups/v3 v3.0.3 // indirect
	github.com/containerd/containerd/api v1.8.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.6 // indirect
	github.com/containerd/typeurl/v2 v2.2.2 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/digitalocean/godo v1.136.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.0.1+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/doclambda/protobufquery v0.0.0-20210317203640-88ffabe06a60 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/expr-lang/expr v1.17.2 // indirect
	github.com/facette/natsort v0.0.0-20181210072756-2cd4dd1e2dcb // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-resty/resty/v2 v2.16.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.3.0 // indirect
	github.com/go-zookeeper/zk v1.0.4 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/cadvisor v0.52.1 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.5 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/gopcua/opcua v0.8.0 // indirect
	github.com/gophercloud/gophercloud v1.14.1 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/gosnmp/gosnmp v1.34.0 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1 // indirect
	github.com/hashicorp/consul/api v1.32.0 // indirect
	github.com/hashicorp/cronexpr v1.1.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/nomad/api v0.0.0-20241218080744-e3ac00f30eec // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/hetznercloud/hcloud-go/v2 v2.19.1 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/influxdata/line-protocol/v2 v2.2.1 // indirect
	github.com/influxdata/toml v0.0.0-20190415235208-270119a8ce65 // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.3.2 // indirect
	github.com/jaegertracing/jaeger v1.62.0 // indirect
	github.com/jaegertracing/jaeger-idl v0.5.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jhump/protoreflect v1.8.3-0.20210616212123-6cc1efa697ca // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/kolo/xmlrpc v0.0.0-20220921171641-a4b6fa1dd06b // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/go-syslog/v4 v4.2.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/linode/linodego v1.47.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/magiconair/properties v1.8.9 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mdlayher/socket v0.4.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/miekg/dns v1.1.63 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/naoina/go-stringutil v0.1.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.124.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.124.1 // indirect
	github.com/opencontainers/cgroups v0.0.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/ovh/go-ovh v1.7.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/philhofer/fwd v1.1.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus/alertmanager v0.28.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common/assets v0.2.0 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/exporter-toolkit v0.14.0 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/relvacode/iso8601 v1.6.0 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/safchain/ethtool v0.0.0-20210803160452-9aa261dae9b1 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.32 // indirect
	github.com/shirou/gopsutil/v4 v4.25.3 // indirect
	github.com/shurcooL/httpfs v0.0.0-20230704072500-f1e31cf0ba5c // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/sleepinggenius2/gosmi v0.4.4 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.1.0 // indirect
	github.com/tidwall/wal v1.1.8 // indirect
	github.com/tinylib/msgp v1.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/vjeantet/grok v1.0.1 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/wavefronthq/wavefront-sdk-go v0.9.10 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.etcd.io/bbolt v1.4.0 // indirect
	go.mongodb.org/mongo-driver v1.15.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector v0.124.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.124.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.30.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.124.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.30.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.30.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.30.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.30.0 // indirect
	go.opentelemetry.io/collector/connector v0.124.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.124.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.124.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.124.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.124.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.124.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.124.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.124.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.30.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.124.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.124.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.30.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.124.0 // indirect
	go.opentelemetry.io/collector/internal/memorylimiter v0.124.0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.124.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.124.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.124.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.124.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.124.0 // indirect
	go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.124.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.124.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.124.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.124.0 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.124.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.15.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.35.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.57.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	golang.org/x/tools v0.32.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/api v0.226.0 // indirect
	google.golang.org/genproto v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250428153025-10db94c68c34 // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	k8s.io/kubelet v0.30.0 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	modernc.org/sqlite v1.21.2 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
