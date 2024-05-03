# Delta To Sparse Processor

## Description

The delta to sparse processor (`deltatosparseprocessor`) drops 0 valued datapoints of monotonic, delta sum and histogram metrics making them sparse. Only values !=0 values are retained.

## Configuration

Configuration is specified through a list of metrics. The processor uses metric names to identify a set of cumulative metrics and converts them from cumulative to delta.

The following settings can be optionally configured:

- `include`: List of metrics names to convert to delta.

If include list is not supplied then no filtering is applied.

#### Examples

```yaml
processors:
    # processor name: deltatosparse
    deltatosparse:

        # list the exact cumulative sum or histogram metrics to convert to delta
        include:
            - <metric_1_name>
            - <metric_2_name>
            .
            .
            - <metric_n_name>
```

```yaml
processors:
    # processor name: deltatosparse
    deltatosparse:
        # If include/exclude are not specified
        # no datapoints are dropped from any metrics
```