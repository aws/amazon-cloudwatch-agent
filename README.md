# Overview

CloudWatchAgent (CWA) is an agent which collects system-level metrics, custom metrics (e.g Prometheus, Statsd, Collectd), monitoring logs and publicizes these telemetry data to AWS CloudWatch Metrics, and Logs backends. It is fully compatible with AWS computing platforms including EC2, ECS, and EKS and non-AWS environment.

See the [Amazon CloudWatch Agent](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html) for more information on supported OS and how to install Amazon CloudWatch Agent.

# Components and Use Case

### CWA Built-in Components

| Input             | Processor    | Output         |
| ----------------- | ------------ | -------------- |
| cpu               | ec2tagger    | cloudwatch     |
| disk              | delta        | cloudwatchlogs |
| diskio            | ecsdecorator |                |
| ethtool           | emfProcessor |                |
| mem               | k8sdecorator |                |
| net               |              |                |
| nvidia_smi        |              |                |
| processes         |              |                |
| procstat          |              |                |
| collectd          |              |                |
| emf               |              |                |
| prometheus        |              |                |
| awscsm            |              |                |
| cadvisor          |              |                |
| k8sapiserver      |              |                |
| logfile           |              |                |
| windows_event_log |              |                |
| win_perf_counters |              |                |

### CWA Use Case

-   [ECS Container Insight](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/deploy-container-insights-ECS.html)
-   [EKS Container Insight](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/deploy-container-insights-EKS.html)
-   [Prometheus](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-install-EKS.html)
-   [EMF](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Generation_CloudWatch_Agent.html)
-   [Collectd](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-custom-metrics-collectd.html)
-   [Statsd](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-custom-metrics-statsd.html)

# Getting start

### Prerequisites

To build the Amazon CloudWatch Agent locally, you will need to have Golang installed. You can download and install [Golang](https://go.dev/doc/install)

### CWA configuration

Amazon CloudWatch Agent is built with a [default configuration](https://github.com/aws/amazon-cloudwatch-agent/blob/main/translator/config/defaultConfig.go#L6-L176).The Amazon CloudWatch Agent uses the JSON configuration following [this schema design](https://github.com/aws/amazon-cloudwatch-agent/blob/main/translator/config/schema.json). For more information on how to configure Amazon CloudWatchAgent configuration files when running the agent, please following this [document](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Configuration-File-Details.html).

### Try out CWA

The Amazon CloudWatch Agent supports all AWS computing platforms and Docker/Kubernetes. Here are some examples on how to run the Amazon CloudWatch Agent to send telemetry data:

-   [Run in with local host](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Agent-on-premise.html)
-   [Run in with EC2](https://docs.amazonaws.cn/en_us/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Agent-on-EC2-Instance-fleet.html)
-   [Run in with ECS](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-Setup-ECS.html)
-   [Run in with EKS](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-install-EKS.html)

### Build your own artifacts

Use the following instructions to build your own Amazon Cloudwatch Agent artifacts:

-   [Build RPM/DEB/MSI/TAR](https://github.com/aws/amazon-cloudwatch-agent/tree/main#building-and-running-from-source)
-   [Build Docker image](https://github.com/aws/amazon-cloudwatch-agent/tree/main/amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile)

# Getting help

Use the community resources below for getting help with the Amazon CloudWatch Agent.

-   Use GitHub issues to [report bugs and request features](https://github.com/aws/amazon-cloudwatch-agent/issues/new/choose).
-   If you think you may have found a security issues, please following this [instruction](https://aws.amazon.com/security/vulnerability-reporting/).
-   For contributing guidelines, refer to [CONTRIBUTING.md](https://github.com/aws/amazon-cloudwatch-agent/blob/main/CONTRIBUTING.md).

# License

MIT License
Copyright (c) 2015-2019 InfluxData Inc. Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved. Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the Software), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions: The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
