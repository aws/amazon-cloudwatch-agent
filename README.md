[![codecov](https://codecov.io/gh/aws/amazon-cloudwatch-agent/branch/main/graph/badge.svg?token=79VYANUGOM)](https://codecov.io/gh/aws/amazon-cloudwatch-agent)

# Amazon CloudWatch Agent
The Amazon CloudWatch Agent is software developed for the [CloudWatch Agent](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html)

## Overview
The Amazon CloudWatch Agent enables you to do the following:

- Collect more system-level metrics from Amazon EC2 instances across operating systems. The metrics can include in-guest metrics, in addition to the metrics for EC2 instances. The additional metrics that can be collected are listed in [Metrics Collected by the CloudWatch Agent](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/metrics-collected-by-CloudWatch-agent.html).
- Collect system-level metrics from on-premises servers. These can include servers in a hybrid environment as well as servers not managed by AWS.
- Retrieve custom metrics from your applications or services using the StatsD and collectd protocols. StatsD is supported on both Linux servers and servers running Windows Server. collectd is supported only on Linux servers.
- Collect logs from Amazon EC2 instances and on-premises servers, running either Linux or Windows Server.
- Collect Open Telemetry and AWS X-Ray traces

Amazon CloudWatch Agent uses open-source projects [telegraf](https://github.com/influxdata/telegraf) and [opentelemetry-collector](https://github.com/open-telemetry/opentelemetry-collector) as its dependencies. 
It operates by starting an opentelemetry collector and is capable of operating pipelines consisting of both telegraf and opentemetry components in addition to customized components.

### Setup
* [Configuring IAM Roles](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/create-iam-roles-for-cloudwatch-agent.html)
* [Installation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Agent-on-EC2-Instance.html)
* [Configuring the CloudWatch Agent](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/create-cloudwatch-agent-configuration-file.html)

### Troubleshooting
* [Troubleshooting CloudWatch Agent](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/troubleshooting-CloudWatch-Agent.html)

## Building and Running from source

* Install go. For more information, see [Getting started](https://golang.org/doc/install)
* The agent uses go modules for dependency management. For more information, see [Go Modules](https://github.com/golang/go/wiki/Modules)

* Install rpm-build
```
sudo yum install -y rpmdevtools rpm-build
```
* Run `make build` to build the CloudWatch Agent for Linux, Debian, Windows environment.

* Run `make release` to build the agent. This also packages it into a RPM, DEB and ZIP package.

The following folders are generated when the build completes:
```
build/bin/linux/arm64/amazon-cloudwatch-agent.rpm
build/bin/linux/amd64/amazon-cloudwatch-agent.rpm
build/bin/linux/arm64/amazon-cloudwatch-agent.deb
build/bin/linux/amd64/amazon-cloudwatch-agent.deb
build/bin/windows/amd64/amazon-cloudwatch-agent.zip
build/bin/darwin/amd64/amazon-cloudwatch-agent.tar.gz
```

* Install your own build of the agent

    1. rpm package

        * `rpm -Uvh amazon-cloudwatch-agent.rpm`

    1. deb package

        * `dpkg -i -E ./amazon-cloudwatch-agent.deb`

    1. windows package

        * unzip `amazon-cloudwatch-agent.zip`
        * `./install.ps1`

    1. darwin package
        * `tar -xvf amazon-cloudwatch-agent.tar.gz`
        * `cp -rf ./opt/aws /opt`
        * `cp -rf ./Library/LaunchDaemons/com.amazon.cloudwatch.agent.plist /Library/LaunchDaemons/`

### Building and running container

See [Dockerfiles](amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile).

### Make Targets
The following targets are available. Each may be run with `make <target>`.

| Make Target              | Description |
|:-------------------------|:------------|
| `build`                  | `build` builds the agent for Linux, Debian and Windows amd64 environment |
| `release`                | *(Default)* `release` builds the agent and also packages it into a RPM, DEB and ZIP package |
| `clean`                  | `clean` removes build artifacts |
| `dockerized-build`       | build using docker container without local go environment |

## Features
### Log Filtering
CloudWatch agent supports log filtering, where the agent processes each log message with the filters that you specify, and only published events that pass all filters to CloudWatch Logs. See [docs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Configuration-File-Details.html#CloudWatch-Agent-Configuration-File-Logssection) for details.

For example, the following excerpt of the CloudWatch agent configuration file publishes logs that are PUT and POST requests to CloudWatch Logs, but excluding logs that come from Firefox:
```json
{
  "collect_list": [
    {
      "file_path": "/opt/aws/amazon-cloudwatch-agent/logs/test.log",
      "log_group_name": "test.log",
      "log_stream_name": "test.log",
      "filters": [
        {
          "type": "exclude",
          "expression": "Firefox"
        },
        {
          "type": "include",
          "expression": "P(UT|OST)"
        }
      ]
    }
  ]
}
```
Example with above config:
```
2021-09-27T19:36:35Z I! [logagent] Firefox Detected   // Agent excludes this 
2021-09-27T19:36:35Z POST (StatusCode: 200).  // Agent would push this to CloudWatch
2021-09-27T19:36:35Z GET (StatusCode: 400). // doesn't match regex, will be excluded
```
## Versioning
It is using [Semantic versioning](https://semver.org/)

## Distributions
You can download the official release from S3, refer to [link](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/download-cloudwatch-agent-commandline.html)

Nightly s3 release are not production ready and should be used at own risk
1. Download Binaries
    1. Linux
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/linux_{amd64/arm64}/amazon-cloudwatch-agent
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/linux_{amd64/arm64}/amazon-cloudwatch-agent-config-wizard
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/linux_{amd64/arm64}/config-downloader
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/linux_{amd64/arm64}/config-translator
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/linux_{amd64/arm64}/start-amazon-cloudwatch-agent
    1. Windows
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/windows_amd64/amazon-cloudwatch-agent-config-wizard.exe
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/windows_amd64/amazon-cloudwatch-agent.exe
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/windows_amd64/config-downloader.exe
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/windows_amd64/config-translator.exe
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/windows_amd64/start-amazon-cloudwatch-agent.exe
    1. Mac
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/darwin_amd64/amazon-cloudwatch-agent
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/darwin_amd64/amazon-cloudwatch-agent-config-wizard
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/darwin_amd64/config-downloader
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/darwin_amd64/config-translator
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/darwin_amd64/start-amazon-cloudwatch-agent
2. Download Packages
    1. Linux
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/linux/{amd64/arm64}/amazon-cloudwatch-agent.{deb/rpm}
    2. Windows
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/windows/amd64/amazon-cloudwatch-agent.zip
    3. Mac
        * https://amazoncloudwatch-agent.s3.amazonaws.com/nightly-build/latest/darwin/amd64/amazon-cloudwatch-agent.tar.gz

## Usage data
By default, the CloudWatch agent sends health and performance data about itself to CloudWatch whenever it publishes metrics or logs to CloudWatch. This data incurs no costs to you. You can prevent the agent from sending this data by specifying `false` for `usage_data` in the `agent` section of the configuration. If you omit this parameter, the default of true is used and the agent sends the health and performance data. Refer to [link](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Configuration-File-Details.html#CloudWatch-Agent-Configuration-File-Agentsection).

## Security disclosures
If you think you’ve found a potential security issue, please do not post it in the Issues.  Instead, please follow the instructions [here](https://aws.amazon.com/security/vulnerability-reporting/) or [email AWS security directly](mailto:aws-security@amazon.com).

## License

MIT License

Copyright (c) 2015-2019 InfluxData Inc.
Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including  without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to  the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN  NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE  SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.