# Rollup Processor

The Rollup Processor creates new data points with attribute sets that are aggregated (rolled up) from the original data points.
For example, specifying an attribute set of `["Attr1","Attr2"]` would roll up on those two attributes, creating a new data point
with only `"Attr1"` and `"Attr2"` and dropping all other attributes.

| Status                   |                           |
| ------------------------ |---------------------------|
| Stability                | [beta]                    |
| Supported pipeline types | metrics                   |
| Distributions            | [amazon-cloudwatch-agent] |

The attribute groups obtain their values from the original data point. If the data point does not have
the configured attribute, then that group will not be created. This data point roll up can provide
an exporter with the capability of aggregating the metrics based on these groups. The processor also
supports dropping the original data point to reduce the amount of data being sent along the pipeline.

### Processor Configuration:

The following processor configuration parameters are supported.

| Name               | Description                                                                            | Supported Value                                    | Default |
|--------------------|----------------------------------------------------------------------------------------|----------------------------------------------------|---------|
| `attribute_groups` | The groups of attribute names that will be used to create the rollup data points with. | [["Attribute1", "Attribute2"], ["Attribute1"], []] | []      |
| `drop_original`    | The names of metrics where the original data points should be dropped.                 | ["MetricName1", "MetricName2"]                     | []      |
| `cache_size`       | The size of the rollup cache used for optimization. Can be disabled by setting to <= 0 | 100                                                | 1000    |
