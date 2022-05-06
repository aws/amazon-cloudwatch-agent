module github.com/aws/amazon-cloudwatch-agent

go 1.13

replace github.com/influxdata/telegraf => github.com/aws/telegraf v0.10.2-0.20201015165757-4470de2d306b

replace github.com/shirou/gopsutil => github.com/aws/telegraf/patches/gopsutil v0.0.0-20201015165757-4470de2d306b // Temporary fix, pending PR https://github.com/shirou/gopsutil/pull/957

//pin consul to a newer version to fix the ambiguous import issue
//see https://github.com/hashicorp/consul/issues/6019 and https://github.com/hashicorp/consul/issues/6414
//Consul is used only by an plugin in telegraf and the version changes from v1.2.1 to v1.8.4 here (no major version change)
//so the replacement should not affect amazon-cloudwatch-agent
replace github.com/hashicorp/consul => github.com/hashicorp/consul v1.8.4

//hashicorp/go-discover depends on tencentcloud-sdk-go. Tencent has downgraded their versioning from
//v3.x.xx to v1.x.xx (see https://github.com/TencentCloud/tencentcloud-sdk-go/issues/103). There's an open
//PR at hashicorp/go-discover (see https://github.com/hashicorp/go-discover/pull/173), but it's yet to be
//merged. In the meantime, this is a workaround.
replace github.com/tencentcloud/tencentcloud-sdk-go => github.com/tencentcloud/tencentcloud-sdk-go v1.0.83

//consul@v1.8.4 points to a commit in go-discover that depends on an older version of kubernetes: kubernetes-1.16.9
//https://github.com/hashicorp/consul/blob/12b16df320052414244659e4dadda078f67849ed/go.mod#L38
//This commit contains a dependency in launchpad.net which requires the version control system Bazaar to be set up
//https://github.com/hashicorp/go-discover/commit/ad1e96bde088162a25dc224d687440181b704162#diff-37aff102a57d3d7b797f152915a6dc16R44
//To avoid the requirement for Bazaar, we want to replace go-discover with a newer version. However to avoid the upgrade of k8s.io lib from
//0.17.4 (used by current amazon-cloudwatch-agent) to 0.18.x, we choose the commit just before go-discover introduced k8s.io 0.18.8
//go-discover is used only by consul and consul is used only in telegraf, so the replacement here should not affect amazon-cloudwatch-agent
replace github.com/hashicorp/go-discover => github.com/hashicorp/go-discover v0.0.0-20200713171816-3392d2f47463

//to be consistent with prometheus: https://github.com/prometheus/prometheus/blob/18254838fbe25dcc732c950ae05f78ed4db1292c/go.mod#L62
replace k8s.io/klog => github.com/simonpasquier/klog-gokit v0.1.0

//proxy.golang.org has versions of golang.zx2c4.com/wireguard with leading v's, whereas the git repo has tags without
//leading v's: https://git.zx2c4.com/wireguard-go/refs/tags
//So, fetching this module with version v0.0.20200121 (as done by the transitive dependency
//https://github.com/WireGuard/wgctrl-go/blob/e35592f146e40ce8057113d14aafcc3da231fbac/go.mod#L12 ) was not working when
//using GOPROXY=direct.
//Replacing with the pseudo-version works around this.
replace golang.zx2c4.com/wireguard v0.0.20200121 => golang.zx2c4.com/wireguard v0.0.0-20200121152719-05b03c675090

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Jeffail/gabs v1.4.0
	github.com/aws/aws-sdk-go v1.30.15
	github.com/aws/aws-sdk-go-v2 v1.16.3
	github.com/aws/aws-sdk-go-v2/config v1.15.3
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.3
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.18.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.15.4
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.29.0
	github.com/aws/smithy-go v1.11.2
	github.com/bigkevmcd/go-configparser v0.0.0-20200217161103-d137835d2579
	github.com/docker/docker v1.13.1
	github.com/go-kit/kit v0.10.0
	github.com/gobwas/glob v0.2.3
	github.com/google/cadvisor v0.36.0
	github.com/google/go-cmp v0.5.7
	github.com/hashicorp/golang-lru v0.5.4
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/influxdata/telegraf v0.0.0-00010101000000-000000000000
	github.com/influxdata/toml v0.0.0-20190415235208-270119a8ce65
	github.com/influxdata/wlog v0.0.0-20160411224016-7c63b0a71ef8
	github.com/kardianos/service v1.0.0
	github.com/kr/pretty v0.2.0
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/prometheus/prometheus v1.8.2-0.20200420081721-18254838fbe2
	github.com/shirou/gopsutil v2.20.5+incompatible
	github.com/stretchr/testify v1.7.0
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20200316230553-a7d97aace0b0
	golang.org/x/text v0.3.3
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/klog v1.0.0
)
