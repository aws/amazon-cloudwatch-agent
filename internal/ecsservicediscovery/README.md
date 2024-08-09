## ECS Prometheus Exporter Auto Discovery

### Overview
This module provides the Prometheus exporter auto discovery functionality based on the ECS task metadata.

There are two modes to discover the Prometheus targets based on the customer config:
* **Mode 1: Docker label Based:** Customers add the docker labels to the containers to indicate the port and metric path of the Prometheus metrics. Customers configure the CWAgent to discover the Prometheus targets with the matching docker label and container port.
* **Mode 2: ECS Task Definition ARN based:**  Customers configure the CWAgent to discover the Prometheus targets when its ECS task definition arn matches the configured regex and it matches the configured container ports.

Two modes can be enabled together and CWAgent will de-dup the discovered targets based on: *{private_ip}:{port}/{metrics_path}*

#### Service Discovery Workflow

1. List the current running ECS task ARNs for the specific ECS cluster by `ECS:ListTasks paginated call`
2. Describe the ECS tasks based on the ListTasks response `ECS:DescribeTasks batch call`
3. Get the ECS Task Definition from LRU cache, if there is none in cache, call `ECS:DescribeTaskDefinition` and cache. LRU cache size (2000) based on [ECS service quota](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html)
4. Check the Container Docker Label if there is Docker Label based Service Discovery config
5. Check the Task Definition ARN if there is Task Definition ARN Regex config
6. Filter the ECS tasks that match the above two checking for further processing
7. Get the containerInstance/ec2 instance info from LRU cache if the tasks is running on EC2 launch type. LRU cache size (2000) based on [ECS service quota](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html)
8. Call `ECS: DescribeContainerInstances` and `EC2:DescribeInstances` for the instances have not been cached. Batching Call with batch size = 100.
9. Export the ECS Prometheus targets into file configured by `sd_result_file`

### Configuration Options


#### Overall Configuration Options

|Configuration Field  |             | Description                                                    |
|---------------------|-------------|----------------------------------------------------------------|
|sd_frequency         | Mandatory   | frequency to discover the prometheus exporters                 |
|sd_target_cluster    | Mandatory   | target ECS cluster name for service discovery                  |
|sd_cluster_region    | Mandatory   | the target ECS clusters' AWS region name                       |
|sd_result_file       | Mandatory   | path of the yaml file for the Prometheus target results        |
|docker_label         | Optional    | docker label based service discovery configurations. If this structure is nil, docker label based service discovery is disabled                |
|task_definition_list | Optional    | ECS task definition based service discovery configurations slice. If this slice is empty, task definition based service discovery is disabled  |

#### Service Endpoint Based Auto Discovery

|Configuration Field  |             | Description                                                   |
|---------------------|-------------|---------------------------------------------------------------|
|sd_service_name_pattern         | Mandatory   | ECS service name regex pattern             |
|sd_metrics_ports                | Mandatory   | semicolon separated containerPort for Prometheus metrics.    |
|sd_container_name_pattern       | Optional    | ECS task container name regex pattern                        |
|sd_metrics_path                 | Optional    | Prometheus metric path. If not specified, the default path /metrics is assumed        |
|sd_job_name                     | Optional    | Prometheus scrape job name. If not specified, the job name in prometheus.yaml is used   |

#### Docker Label Based Auto Discovery Configuration

|Configuration Field  |             | Description                                                   |
|---------------------|-------------|---------------------------------------------------------------|
|sd_port_label        | Mandatory   | Container's docker label name that specify the containerPort for Prometheus metrics. Only one value is allowed                          |
|sd_metrics_path_label| Optional    | Container's docker label name that specify the Prometheus metric path. If not specified, the default path /metrics is assumed.          |
|sd_job_name_label    | Optional    | Container's docker label name that specify the Prometheus scrape job name. If not specified, the job name in prometheus.yaml is used.   |


#### Task Definition Based Auto Discovery

|Configuration Field  |             | Description                                                   |
|---------------------|-------------|---------------------------------------------------------------|
|sd_task_definition_arn_pattern  | Mandatory   | ECS task definition arn regex pattern             |
|sd_metrics_ports                | Mandatory   | semicolon separated containerPort for Prometheus metrics.    |
|sd_container_name_pattern       | Optional    | ECS task container name regex pattern                        |
|sd_metrics_path                 | Optional    | Prometheus metric path. If not specified, the default path /metrics is assumed        |
|sd_job_name                     | Optional    | Prometheus scrape job name. If not specified, the job name in prometheus.yaml is used   |


#### Configuration Example
Sample Configuration in TOML format:
```
    [inputs.prometheus.ecs_service_discovery]
      sd_cluster_region = "us-east-2"
      sd_frequency = "15s"
      sd_result_file = "/opt/aws/amazon-cloudwatch-agent/etc/ecs_sd_targets.yaml"
      sd_target_cluster = "EC2-Justin-Testing"
      [inputs.prometheus.ecs_service_discovery.docker_label]
        sd_job_name_label = "ECS_PROMETHEUS_JOB_NAME"
        sd_metrics_path_label = "ECS_PROMETHEUS_METRICS_PATH"
        sd_port_label = "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_A"

      [[inputs.prometheus.ecs_service_discovery.task_definition_list]]
        sd_job_name = "task_def_1"
        sd_metrics_path = "/stats/metrics"
        sd_metrics_ports = "9901;9404;9406"
        sd_task_definition_arn_pattern = ".*:task-definition/bugbash-java-fargate-awsvpc-task-def-only:[0-9]+"

      [[inputs.prometheus.ecs_service_discovery.task_definition_list]]
        sd_container_name_pattern = "^bugbash-jar.*$"
        sd_metrics_ports = "9902"
        sd_task_definition_arn_pattern = ".*:task-definition/nginx:[0-9]+"
```


### Permission
ECS Task Role needs to be granted the following permission so CWAgent can query the ECS/EC2 frontend to get the task meteData.
* **ECS Policy**
```
ECS:ListTasks,
ECS:DescribeContainerInstances,
ECS:DescribeTasks,
ECS:DescribeTaskDefinition
EC2:DescribeInstances
```

## Example Result

```yaml
- targets:
  - 10.6.1.95:32785
  labels:
    __metrics_path__: /metrics
    ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_B: "9406"
    ECS_PROMETHEUS_JOB_NAME: demo-jar-ec2-bridge-subset-b-dynamic
    ECS_PROMETHEUS_METRICS_PATH: /metrics
    InstanceType: t3.medium
    LaunchType: EC2
    SubnetId: subnet-0347624eeea6c5969
    TaskDefinitionFamily: demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskGroup: family:demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskRevision: "7"
    VpcId: vpc-033b021cd7ecbcedb
    container_name: demo-jar-ec2-bridge-dynamic-port-subset-b
    job: task_def_2
- targets:
  - 10.6.1.95:32783
  labels:
    __metrics_path__: /metrics
    ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_B: "9406"
    ECS_PROMETHEUS_JOB_NAME: demo-jar-ec2-bridge-subset-b-dynamic
    ECS_PROMETHEUS_METRICS_PATH: /metrics
    InstanceType: t3.medium
    LaunchType: EC2
    SubnetId: subnet-0347624eeea6c5969
    TaskDefinitionFamily: demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskGroup: family:demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskRevision: "7"
    VpcId: vpc-033b021cd7ecbcedb
    container_name: demo-jar-ec2-bridge-dynamic-port-subset-b
    job: task_def_2
```

