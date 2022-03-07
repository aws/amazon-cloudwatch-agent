#!/usr/bin/env bash

cd "$(dirname "$0")"

newversion="k8s\/1.0.1"

sed -i'.bak' "s/k8s\/[0-9]*\.[0-9]*\.[0-9]*/$newversion/g" ./cwagent-kubernetes-monitoring/cwagent-daemonset.yaml
rm ./cwagent-kubernetes-monitoring/cwagent-daemonset.yaml.bak

sed -i'.bak' "s/k8s\/[0-9]*\.[0-9]*\.[0-9]*/$newversion/g" ./fluentd/fluentd.yaml
rm ./fluentd/fluentd.yaml.bak

OUTPUT=./quickstart/cwagent-fluentd-quickstart.yaml

cat ./cloudwatch-namespace.yaml >${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ./cwagent-kubernetes-monitoring/cwagent-serviceaccount.yaml >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ./cwagent-kubernetes-monitoring/cwagent-configmap.yaml | sed "s|\"logs|\"agent\": {\\
        \"region\": \"{{region_name}}\"\\
      },\\
      \"logs|g" >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ./cwagent-kubernetes-monitoring/cwagent-daemonset.yaml >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ./fluentd/fluentd-configmap.yaml >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ./fluentd/fluentd.yaml >>${OUTPUT}
