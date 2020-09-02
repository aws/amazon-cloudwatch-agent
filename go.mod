module github.com/aws/amazon-cloudwatch-agent

go 1.13

replace github.com/influxdata/telegraf => github.com/aws/telegraf v0.10.2-0.20200902215110-5ec6811a19d9

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Jeffail/gabs v1.4.0
	github.com/aws/aws-sdk-go v1.30.15
	github.com/bigkevmcd/go-configparser v0.0.0-20200217161103-d137835d2579
	github.com/docker/docker v1.13.1
	github.com/gobwas/glob v0.2.3
	github.com/google/cadvisor v0.36.0
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/influxdata/telegraf v1.15.2
	github.com/influxdata/toml v0.0.0-20190415235208-270119a8ce65
	github.com/influxdata/wlog v0.0.0-20160411224016-7c63b0a71ef8
	github.com/kardianos/service v1.0.0
	github.com/opencontainers/runc v1.0.0-rc10
	github.com/shirou/gopsutil v2.20.5+incompatible
	github.com/stretchr/testify v1.5.1
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20200212091648-12a6c2dcc1e4
	golang.org/x/text v0.3.2
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200322164244-327a8059b905 // indirect
)
