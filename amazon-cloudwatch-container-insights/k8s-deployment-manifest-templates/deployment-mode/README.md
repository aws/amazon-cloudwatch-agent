## Example Kubernetes YAML files based on deployment modes

You can deploy CloudWatch Agent into Kubernetes Cluster for some various functionality: StatsD, CloudWatch EMF, AWS SDK Metrics, etc.

For each of these features, you can deploy the CloudWatch Agent in one or more of the following deployment modes:
* Sidecar - CloudWatch Agent works at the Pod level. Reasons for choosing this mode include but are not restricted to:
  * You expect a dedicated CloudWatch Agent for your Pod. 
* Daemonset - CloudWatch Agent works at the Node level. Reasons for choosing this mode include but are not restricted to:
  * You expect CloudWatch Agent to serve for all Pods within a Node.
  * You expect CloudWatch Agent to collect metrics for each Node.
  * You expect CloudWatch Agent to be scaled according to the number of Node automatically.
* Service - CloudWatch Agent works at the Cluster level. Reasons for choosing this mode include but are not restricted to:
  * You expect CloudWatch Agent to serve for all Pods within a Cluster.
  * You expect an centralized endpoint within a cluster for your application to access.
  * You expect an application outside the cluster to be able to access to CloudWatch Agent.

You can choose the mode that is most suitable for you.

This folder contains example Kubernetes YAML files for various functionality based on deployment modes.

The following subfolders contain example Amazon ECS task definition files based on the deployment modes:

* [sidecar](sidecar)
* [service](service)
* [daemonset](daemonset)