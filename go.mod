module github.com/aws/amazon-cloudwatch-agent

go 1.22.0

toolchain go1.22.2

replace github.com/influxdata/telegraf => github.com/aws/telegraf v0.10.2-0.20240423220441-63baeaedb379

// Replace with https://github.com/amazon-contributing/opentelemetry-collector-contrib, there are no requirements for all receivers/processors/exporters
// to be all replaced since there are some changes that will always be from upstream
replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter => github.com/amazon-contributing/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter => github.com/amazon-contributing/opentelemetry-collector-contrib/exporter/awsemfexporter v0.0.0-20240517183704-e0e66ca9e79c
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter => github.com/amazon-contributing/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.0.0-20240517183704-e0e66ca9e79c
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy => github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsproxy v0.0.0-20240503173519-cc2b921759f4

replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/awsutil v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/containerinsight v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/cwlogs v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/k8s v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/proxy v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/aws/xray v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet => github.com/amazon-contributing/opentelemetry-collector-contrib/internal/kubelet v0.0.0-20240503173519-cc2b921759f4
)

replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => github.com/amazon-contributing/opentelemetry-collector-contrib/pkg/stanza v0.0.0-20240503173519-cc2b921759f4
	// Replace with contrib to revert upstream change https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/20519
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => github.com/amazon-contributing/opentelemetry-collector-contrib/pkg/translator/prometheus v0.0.0-20240503173519-cc2b921759f4
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => github.com/amazon-contributing/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.0.0-20240503173519-cc2b921759f4

replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.0.0-20240503173519-cc2b921759f4
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => github.com/amazon-contributing/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.0.0-20240503173519-cc2b921759f4
)

// Temporary fix, pending PR https://github.com/shirou/gopsutil/pull/957
replace github.com/shirou/gopsutil/v3 => github.com/aws/telegraf/patches/gopsutil/v3 v3.0.0-20231109213610-a8c21c54a2be // indirect

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
	github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware v0.0.0-20240503173519-cc2b921759f4
	github.com/aws/aws-sdk-go v1.51.17
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.30.2
	github.com/bigkevmcd/go-configparser v0.0.0-20200217161103-d137835d2579
	github.com/deckarep/golang-set/v2 v2.3.1
	github.com/go-kit/log v0.2.1
	github.com/go-test/deep v1.0.2-0.20181118220953-042da051cf31
	github.com/gobwas/glob v0.2.3
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/hashicorp/golang-lru v1.0.2
	github.com/influxdata/telegraf v0.0.0-00010101000000-000000000000
	github.com/influxdata/wlog v0.0.0-20160411224016-7c63b0a71ef8
	github.com/jellydator/ttlcache/v3 v3.2.0
	github.com/kardianos/service v1.2.1 // Keep this pinned to v1.2.1. v1.2.2 causes the agent to not register as a service on Windows
	github.com/kr/pretty v0.3.1
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c
	github.com/oklog/run v1.1.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver v0.98.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver v0.98.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.19.0
	github.com/prometheus/common v0.52.2
	github.com/prometheus/prometheus v0.51.2-0.20240405174432-b4a973753c6e
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/shirou/gopsutil/v3 v3.24.3
	github.com/stretchr/testify v1.9.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.opentelemetry.io/collector/component v0.98.0
	go.opentelemetry.io/collector/config/configopaque v1.5.0
	go.opentelemetry.io/collector/config/configtelemetry v0.98.0
	go.opentelemetry.io/collector/config/configtls v0.98.0
	go.opentelemetry.io/collector/confmap v0.98.0
	go.opentelemetry.io/collector/consumer v0.98.0
	go.opentelemetry.io/collector/exporter v0.98.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.98.0
	go.opentelemetry.io/collector/extension v0.98.0
	go.opentelemetry.io/collector/otelcol v0.98.0
	go.opentelemetry.io/collector/pdata v1.5.0
	go.opentelemetry.io/collector/processor v0.98.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.98.0
	go.opentelemetry.io/collector/receiver v0.98.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.98.0
	go.opentelemetry.io/collector/semconv v0.98.0
	go.opentelemetry.io/collector/service v0.98.0
	go.uber.org/atomic v1.11.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a
	golang.org/x/net v0.24.0
	golang.org/x/sync v0.6.0
	golang.org/x/sys v0.19.0
	golang.org/x/text v0.14.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools/v3 v3.2.0
	k8s.io/api v0.30.0
	k8s.io/apimachinery v0.30.0
	k8s.io/client-go v0.30.0
	k8s.io/klog/v2 v2.120.1
)

require (
	go.opentelemetry.io/collector/confmap/converter/expandconverter v0.98.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v0.98.0
	go.opentelemetry.io/collector/featuregate v1.5.0
)

require (
	cloud.google.com/go/compute v1.24.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.4-0.20230617002413-005d2dfb6b68 // indirect
	collectd.org v0.4.0 // indirect
	github.com/Azure/azure-sdk-for-go v67.1.0+incompatible // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.10.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.5.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.5.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.5.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4 v4.3.0 // indirect
	github.com/Azure/go-autorest/autorest v0.11.29 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.23 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.1 // indirect
	github.com/Code-Hex/go-generics-cache v1.3.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.22.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.12.0-rc.3 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/alecthomas/participle v0.4.1 // indirect
	github.com/alecthomas/participle/v2 v2.1.1 // indirect
	github.com/alecthomas/units v0.0.0-20231202071711-9a357b53e9c9 // indirect
	github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws v0.0.0-20240503173519-cc2b921759f4 // indirect
	github.com/antchfx/jsonquery v1.1.5 // indirect
	github.com/antchfx/xmlquery v1.3.9 // indirect
	github.com/antchfx/xpath v1.2.0 // indirect
	github.com/apache/arrow/go/v12 v12.0.1 // indirect
	github.com/apache/arrow/go/v14 v14.0.2 // indirect
	github.com/apache/thrift v0.17.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.23.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.25.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.16.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.25.2 // indirect
	github.com/aws/smithy-go v1.17.0 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/caio/go-tdigest v3.1.0+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/checkpoint-restore/go-criu/v5 v5.3.0 // indirect
	github.com/cilium/ebpf v0.11.0 // indirect
	github.com/cncf/xds/go v0.0.0-20231128003011-0fa0005c9caa // indirect
	github.com/containerd/cgroups/v3 v3.0.3 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/errdefs v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.2 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/digitalocean/godo v1.109.0 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/docker v25.0.5+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/doclambda/protobufquery v0.0.0-20210317203640-88ffabe06a60 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/envoyproxy/go-control-plane v0.12.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.4 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/expr-lang/expr v1.16.3 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/go-resty/resty/v2 v2.11.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.0.0-alpha.1 // indirect
	github.com/go-zookeeper/zk v1.0.3 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/cadvisor v0.49.0 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.2 // indirect
	github.com/gophercloud/gophercloud v1.8.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/gosnmp/gosnmp v1.34.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.0 // indirect
	github.com/hashicorp/consul/api v1.28.2 // indirect
	github.com/hashicorp/cronexpr v1.1.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.4 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/nomad/api v0.0.0-20240306004928-3e7191ccb702 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/hetznercloud/hcloud-go/v2 v2.6.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/influxdata/go-syslog/v3 v3.0.1-0.20230911200830-875f5bc594a4 // indirect
	github.com/influxdata/line-protocol/v2 v2.2.1 // indirect
	github.com/influxdata/toml v0.0.0-20190415235208-270119a8ce65 // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.1.11 // indirect
	github.com/jhump/protoreflect v1.8.3-0.20210616212123-6cc1efa697ca // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/kolo/xmlrpc v0.0.0-20220921171641-a4b6fa1dd06b // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20181214104525-299bdde78165 // indirect
	github.com/linode/linodego v1.30.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/miekg/dns v1.1.58 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.2 // indirect
	github.com/mrunalp/fileutils v0.5.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/naoina/go-stringutil v0.1.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.98.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runc v1.1.12 // indirect
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	github.com/opencontainers/selinux v1.11.0 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/ovh/go-ovh v1.4.3 // indirect
	github.com/philhofer/fwd v1.1.1 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/rs/cors v1.10.1 // indirect
	github.com/safchain/ethtool v0.0.0-20210803160452-9aa261dae9b1 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.25 // indirect
	github.com/seccomp/libseccomp-golang v0.9.2-0.20220502022130-f33da4d89646 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/sleepinggenius2/gosmi v0.4.4 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tidwall/gjson v1.10.2 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tinylib/msgp v1.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/vjeantet/grok v1.0.1 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/wavefronthq/wavefront-sdk-go v0.9.10 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.5.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.98.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.98.0 // indirect
	go.opentelemetry.io/collector/config/confignet v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configretry v0.98.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpsprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/connector v0.98.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.98.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.98.0 // indirect
	go.opentelemetry.io/contrib/config v0.4.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.25.0 // indirect
	go.opentelemetry.io/otel v1.25.0 // indirect
	go.opentelemetry.io/otel/bridge/opencensus v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.47.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.25.0 // indirect
	go.opentelemetry.io/otel/metric v1.25.0 // indirect
	go.opentelemetry.io/otel/sdk v1.25.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.25.0 // indirect
	go.opentelemetry.io/otel/trace v1.25.0 // indirect
	go.opentelemetry.io/proto/otlp v1.1.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/mod v0.16.0 // indirect
	golang.org/x/oauth2 v0.18.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.19.0 // indirect
	gonum.org/v1/gonum v0.15.0 // indirect
	google.golang.org/api v0.168.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240401170217-c3f982113cda // indirect
	google.golang.org/grpc v1.63.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/kubelet v0.30.0 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
	modernc.org/sqlite v1.21.2 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws => github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws v0.0.0-20240405185623-56e778998456

replace github.com/amazon-contributing/opentelemetry-collector-contrib/pkg/stanza => github.com/amazon-contributing/opentelemetry-collector-contrib/pkg/stanza v0.0.0-20240405185623-56e778998456
