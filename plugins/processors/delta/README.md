# Delta Processor Plugin

The delta processor plugin computes the delta values between previous metric and current metric.

### Configuration:

```toml
# Compute delta for the metrics that pass through this filter if the metrics contains tag report_delta = "true"
[[processors.delta]]
```

### Tags:

No tags are applied by this processor.

### Examples:
To report delta for an input, you need to add the relevant tags for the input plugin like the following example for disk IO:
```toml
[[processors.delta]]

[[inputs.diskio]]
  [inputs.diskio.tags]
    report_deltas = "true"
    ignored_fields_for_delta = "iops_in_progress"
```

Given the following 3 input metrics:
```
diskio,name=sda1,report_delta=true,ignored_fields_for_delta=iops_in_progress read_bytes=31350272i,write_bytes=2117632i,iops_in_progress=0i 1578326400000000000
diskio,name=sda1,report_delta=true,ignored_fields_for_delta=iops_in_progress read_bytes=31360272i,write_bytes=2118632i,iops_in_progress=1i 1578327400000000000
diskio,name=sda1,report_delta=true,ignored_fields_for_delta=iops_in_progress read_bytes=31390272i,write_bytes=2120632i,iops_in_progress=0i 1578328400000000000
```
the delta processor will strip the additional tags "report_delta" and "ignored_fields_for_delta" and produce 2 output metrics:
```
diskio,name=sda1 read_bytes=10000i,write_bytes=1000i,iops_in_progress=1i 1578327400000000000
diskio,name=sda1 read_bytes=30000i,write_bytes=2000i,iops_in_progress=0i 1578328400000000000
```

* The read_bytes/write_bytes is calculated by using `current_value - previous_value`.
* The output metric uses the same timestamp as the current metric in the input.
* Since the field "iops_in_progress" is ignored, the corresponding field in output also use the same value as the current metric in the inupt.

### Note:
Only the field value types `int64`, `unit64`, and `float64` are supported. If an unsupported value type is used, zero value will be returned as delta.