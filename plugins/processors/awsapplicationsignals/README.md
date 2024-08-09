# AWS AppSignals Processor for Amazon Cloudwatch Agent

The AWS AppSignals processor is used to reduce the cardinality of telemetry metrics and traces before exporting them to CloudWatch Logs via [EMF](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/awsemfexporter) and [X-Ray](github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter) respectively.
It reduces the cardinality of metrics/traces via 3 types of actions, `keep`, `drop` and `replace`, which are configured by users. CloudWatch Agent(CWA) customers will configure these rules with their CWA configurations.

Note: Traces support only `replace` actions and are implicitly pulled from the logs section of the CWA configuration

| Status                   |                           |
| ------------------------ |---------------------------|
| Stability                | [beta]                    |
| Supported pipeline types | metrics, traces           |
| Distributions            | [amazon-cloudwatch-agent] |

## Exporter Configuration

The following exporter configuration parameters are supported.

| Name                                         | Description                                                                                                       | Default |
|:---------------------------------------------|:------------------------------------------------------------------------------------------------------------------|---------|
| `resolvers`                                  | Platform processor is being configured for. Currently supports EKS. EC2 platform will be supported in the future. | [eks]   |
| `rules`                                      | Custom configuration rules used for filtering metrics/traces. Can be of type `drop`, `keep`, `replace`.           | []      |

### rules
The rules section defines the rules (filters) to be applied

| Name           | Description                                                                                                              | Default |
|:---------------|:-------------------------------------------------------------------------------------------------------------------------| --- |
| `selectors`    | List of metrics/traces dimension matchers.                                                                               |  [] |
| `action`       | Action being applied for the specified selector. `keep`, `drop`, `replace`                                               |  "" |
| `rule_name`    | (Optional) Name of rule.                                                                                                 |  [] |
| `replacements` | (Optional) List of metrics/traces replacements to be executed. Based on specified selectors. requires `action = replace` |  [] |

#### selectors
A selectors section defines a matching against the dimensions of incoming metrics/traces.

| Name        | Description                                                   | Default |
|:------------|:--------------------------------------------------------------| ------ |
| `dimension` | Dimension of metrics/traces                                   |   ""    |
| `match`     | glob used for matching values of dimensions                   |   ""   |

### replacements
A replacements section defines a matching against the dimensions of incoming metrics/traces for which value replacements will be done. action must be `replace`

| Name               | Description                                   | Default |
|:-------------------|:----------------------------------------------| ------ |
| `target_dimension` | Dimension to replace                          |   ""   |
| `value`            | Value to replace current dimension value with |   ""   |


## AWS AppSignals Processor Configuration Example

```yaml
awsapplicationsignals:
    resolvers: ["eks"]
    rules:
      - selectors:
          - dimension: Operation
            match: "POST *"
          - dimension: RemoteService
            match: "*"
        action: keep
        rule_name: "keep01"
      - selectors:
           - dimension: Operation
             match: "GET *"
           - dimension: RemoteService
             match: "*"
        action: keep
        rule_name: "keep02"
      - selectors:
           - dimension: Operation
             match: "POST *"
        action: drop
        rule_name: "drop01"
      - selectors:
           - dimension: Operation
             match: "*"
        replacements:
          - target_dimension: RemoteOperation
            value: "This is a test string"
        action: replace
        rule_name: "replace01"
```

## Amazon CloudWatch Agent Configuration Example

```json
{
          "agent": {
            "region": "us-west-2",
            "debug": true
          },
          "traces": {
            "traces_collected": {
              "app_signals": {}
            }
          },
          "logs": {
            "metrics_collected": {
              "app_signals": {
                "rules": [
                  {
                    "selectors": [
                      {
                        "dimension": "Service",
                        "match": "pet-clinic-frontend"
                      },
                      {
                        "dimension": "RemoteService",
                        "match": "customers-service"
                      }
                  ],
                    "action": "keep",
                    "rule_name": "keep01"
                },
                {
                  "selectors": [
                    {
                      "dimension": "Operation",
                      "match": "GET *"
                    }
                ],
                  "action": "drop",
                  "rule_name": "drop01"
                }
              }
            }
          }
        }
```