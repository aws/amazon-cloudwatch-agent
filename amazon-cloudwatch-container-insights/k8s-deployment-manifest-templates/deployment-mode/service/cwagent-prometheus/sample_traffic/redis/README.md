## Sample Redis Application Installation Yaml

Set the namespace for the Redis sample workload
```shell script
REDIS_NAMESPACE=redis-sample
```

Run the following command to install the Sample Redis Application on Amazon EKS or Kubernetes
```shell script
curl https://cwagent-prometheus-yamls-justin.s3-us-west-2.amazonaws.com/redis-traffic-sample.yaml \
| sed "s/{{namespace}}/$REDIS_NAMESPACE/g" \
| kubectl apply -f -
```