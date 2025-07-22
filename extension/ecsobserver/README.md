# ECS Observer Extension

This extension provides service discovery for Amazon ECS (Elastic Container Service) tasks and services. It allows the CloudWatch Agent to discover and scrape Prometheus metrics from containers running in ECS.

## Configuration

The ECS observer extension is configured through the CloudWatch Agent configuration file. Here's an example configuration:

```json
{
  "logs": {
    "metrics_collected": {
      "prometheus": {
        "ecs_service_discovery": {
          "sd_target_cluster": "my-ecs-cluster",
          "sd_cluster_region": "us-west-2",
          "sd_result_file": "/opt/aws/amazon-cloudwatch-agent/etc/ecs_sd_targets.yaml",
          "sd_frequency": "60s",
          "docker_label": {
            "sd_port_label": "ECS_PROMETHEUS_EXPORTER_PORT",
            "sd_metrics_path_label": "ECS_PROMETHEUS_METRICS_PATH",
            "sd_job_name_label": "ECS_PROMETHEUS_JOB_NAME"
          },
          "task_definition_list": [
            {
              "sd_job_name": "task-definition-metrics",
              "sd_metrics_path": "/metrics",
              "sd_metrics_ports": "9090",
              "sd_task_definition_arn_pattern": ".*:task-definition/nginx:[0-9]+",
              "sd_container_name_pattern": "nginx.*"
            }
          ],
          "service_name_list_for_tasks": [
            {
              "sd_job_name": "service-metrics",
              "sd_metrics_path": "/metrics",
              "sd_metrics_ports": "9090;9100",
              "sd_service_name_pattern": ".*nginx.*",
              "sd_container_name_pattern": "nginx.*"
            }
          ]
        }
      }
    }
  }
}
```

### Configuration Parameters

#### Required Parameters

- `sd_target_cluster`: The name of the ECS cluster to discover tasks from.
- `sd_cluster_region`: The AWS region of the ECS cluster.
- `sd_result_file`: The path to the file where the discovered targets will be written.

#### Optional Parameters

- `sd_frequency`: The frequency at which to refresh the discovered targets. Default: `10s`.

#### Docker Label Based Discovery

- `docker_label`: Configuration for discovering targets based on Docker labels.
  - `sd_port_label`: The Docker label that specifies the port to scrape. Default: `ECS_PROMETHEUS_EXPORTER_PORT`.
  - `sd_metrics_path_label`: The Docker label that specifies the metrics path. Default: `ECS_PROMETHEUS_METRICS_PATH`.
  - `sd_job_name_label`: The Docker label that specifies the job name. Default: empty.

#### Task Definition Based Discovery

- `task_definition_list`: A list of task definition configurations for discovering targets.
  - `sd_job_name`: The job name for the discovered targets.
  - `sd_metrics_path`: The metrics path for the discovered targets. Default: `/metrics`.
  - `sd_metrics_ports`: A semicolon-separated list of ports to scrape.
  - `sd_task_definition_arn_pattern`: A regex pattern to match task definition ARNs.
  - `sd_container_name_pattern`: A regex pattern to match container names.

#### Service Name Based Discovery

- `service_name_list_for_tasks`: A list of service name configurations for discovering targets.
  - `sd_job_name`: The job name for the discovered targets.
  - `sd_metrics_path`: The metrics path for the discovered targets. Default: `/metrics`.
  - `sd_metrics_ports`: A semicolon-separated list of ports to scrape.
  - `sd_service_name_pattern`: A regex pattern to match service names.
  - `sd_container_name_pattern`: A regex pattern to match container names.

## Implementation Details

The ECS observer extension is implemented as a wrapper around the OpenTelemetry ECS observer extension. It provides additional logging and integration with the CloudWatch Agent.

The extension periodically queries the ECS API to discover tasks and services that match the configured patterns. It then writes the discovered targets to the specified result file, which can be used by the Prometheus receiver to scrape metrics.
