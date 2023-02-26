#!/usr/bin/env bash

cd "$(dirname "$0")"
k8sDirPrefix="./k8s-deployment-manifest-templates/deployment-mode/daemonset/container-insights-monitoring"
ecsDirPrefix="./ecs-task-definition-templates/deployment-mode/daemon-service/cwagent-ecs-instance-metric"

newK8sVersion="k8s/1.3.0"
agentVersion="amazon/cloudwatch-agent:1.247355.0b252062"
fluentdVersion="fluent/fluentd-kubernetes-daemonset:v1.7.3-debian-cloudwatch-1.0"
fluentBitVersion="amazon/aws-for-fluent-bit:2.10.0"

k8sPrometheusDirPrefix="./k8s-deployment-manifest-templates/deployment-mode/service/cwagent-prometheus"
ecsPrometheusDirPrefix="./ecs-task-definition-templates/deployment-mode/replica-service/cwagent-prometheus"

# replace agent version for ECS Prometheus
sed -i'.bak' "s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*\(-prometheus\)\?|${agentVersion}|g" ${ecsPrometheusDirPrefix}/cwagent-prometheus-task-definition.json
rm ${ecsPrometheusDirPrefix}/cwagent-prometheus-task-definition.json.bak
sed -i'.bak' "s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*\(-prometheus\)\?|${agentVersion}|g" ${ecsPrometheusDirPrefix}/cloudformation-quickstart/cwagent-ecs-prometheus-metric-for-awsvpc.yaml
rm ${ecsPrometheusDirPrefix}/cloudformation-quickstart/cwagent-ecs-prometheus-metric-for-awsvpc.yaml.bak
sed -i'.bak' "s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*\(-prometheus\)\?|${agentVersion}|g" ${ecsPrometheusDirPrefix}/cloudformation-quickstart/cwagent-ecs-prometheus-metric-for-bridge-host.yaml
rm ${ecsPrometheusDirPrefix}/cloudformation-quickstart/cwagent-ecs-prometheus-metric-for-bridge-host.yaml.bak

# replace agent and k8s version for K8s Prometheus
sed -i'.bak' "s|k8s/[0-9]*\.[0-9]*\.[0-9a-z]*\(-prometheus\)\?|${newK8sVersion}|g;s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*\(-prometheus\)\?|${agentVersion}|g" ${k8sPrometheusDirPrefix}/prometheus-eks.yaml
rm ${k8sPrometheusDirPrefix}/prometheus-eks.yaml.bak
sed -i'.bak' "s|k8s/[0-9]*\.[0-9]*\.[0-9a-z]*\(-prometheus\)\?|${newK8sVersion}|g;s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*\(-prometheus\)\?|${agentVersion}|g" ${k8sPrometheusDirPrefix}/prometheus-k8s.yaml
rm ${k8sPrometheusDirPrefix}/prometheus-k8s.yaml.bak

# replace agent version for ECS
sed -i'.bak' "s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*|${agentVersion}|g" ${ecsDirPrefix}/cwagent-ecs-instance-metric.json
rm ${ecsDirPrefix}/cwagent-ecs-instance-metric.json.bak

sed -i'.bak' "s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*|${agentVersion}|g" ${ecsDirPrefix}/cloudformation-quickstart/cwagent-ecs-instance-metric-cfn.json
rm ${ecsDirPrefix}/cloudformation-quickstart/cwagent-ecs-instance-metric-cfn.json.bak

# replace agent, fluentD and fluent-bit version for K8s
sed -i'.bak' "s|k8s/[0-9]*\.[0-9]*\.[0-9a-z]*|${newK8sVersion}|g;s|amazon/cloudwatch-agent:[0-9]*\.[0-9]*\.[0-9a-z]*|${agentVersion}|g" ${k8sDirPrefix}/cwagent/cwagent-daemonset.yaml
rm ${k8sDirPrefix}/cwagent/cwagent-daemonset.yaml.bak

sed -i'.bak' "s|k8s/[0-9]*\.[0-9]*\.[0-9a-z]*|${newK8sVersion}|g;s|fluent/fluentd-kubernetes-daemonset:.*|${fluentdVersion}|g" ${k8sDirPrefix}/fluentd/fluentd.yaml
rm ${k8sDirPrefix}/fluentd/fluentd.yaml.bak

sed -i'.bak' "s|k8s/[0-9]*\.[0-9]*\.[0-9a-z]*|${newK8sVersion}|g;s|amazon/aws-for-fluent-bit.*|${fluentBitVersion}|g" ${k8sDirPrefix}/fluent-bit/fluent-bit.yaml
rm ${k8sDirPrefix}/fluent-bit/fluent-bit.yaml.bak

sed -i'.bak' "s|k8s/[0-9]*\.[0-9]*\.[0-9a-z]*|${newK8sVersion}|g;s|amazon/aws-for-fluent-bit.*|${fluentBitVersion}|g" ${k8sDirPrefix}/fluent-bit/fluent-bit-compatible.yaml
rm ${k8sDirPrefix}/fluent-bit/fluent-bit-compatible.yaml.bak

# generate quickstart manifest for K8s
OUTPUT=${k8sDirPrefix}/quickstart/cwagent-fluentd-quickstart.yaml
OUTPUT_FLUENT_BIT=${k8sDirPrefix}/quickstart/cwagent-fluent-bit-quickstart.yaml

cat ${k8sDirPrefix}/cloudwatch-namespace.yaml >${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ${k8sDirPrefix}/cwagent/cwagent-serviceaccount.yaml >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ${k8sDirPrefix}/cwagent/cwagent-configmap.yaml | sed "s|\"logs|\"agent\": {\\
        \"region\": \"{{region_name}}\"\\
      },\\
      \"logs|g" >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ${k8sDirPrefix}/cwagent/cwagent-daemonset.yaml >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ${k8sDirPrefix}/fluentd/fluentd-configmap.yaml >>${OUTPUT}
echo -e "\n---\n" >>${OUTPUT}
cat ${k8sDirPrefix}/fluentd/fluentd.yaml >>${OUTPUT}

cat ${k8sDirPrefix}/cloudwatch-namespace.yaml >${OUTPUT_FLUENT_BIT}
echo -e "\n---\n" >>${OUTPUT_FLUENT_BIT}
cat ${k8sDirPrefix}/cwagent/cwagent-serviceaccount.yaml >>${OUTPUT_FLUENT_BIT}
echo -e "\n---\n" >>${OUTPUT_FLUENT_BIT}
cat ${k8sDirPrefix}/cwagent/cwagent-configmap.yaml | sed "s|\"logs|\"agent\": {\\
        \"region\": \"{{region_name}}\"\\
      },\\
      \"logs|g" >>${OUTPUT_FLUENT_BIT}
echo -e "\n---\n" >>${OUTPUT_FLUENT_BIT}
cat ${k8sDirPrefix}/cwagent/cwagent-daemonset.yaml >>${OUTPUT_FLUENT_BIT}
echo -e "\n---\n" >>${OUTPUT_FLUENT_BIT}
cat ${k8sDirPrefix}/fluent-bit/fluent-bit-configmap.yaml >>${OUTPUT_FLUENT_BIT}
echo -e "\n---\n" >>${OUTPUT_FLUENT_BIT}
cat ${k8sDirPrefix}/fluent-bit/fluent-bit.yaml >>${OUTPUT_FLUENT_BIT}
