## Example Amazon ECS task definitions based on deployment modes

You can deploy CloudWatch Agent into Amazon ECS clusters to enable various functionality: StatsD, AWS SDK Metrics, etc.

For each of these features, you can deploy the CloudWatch Agent in one or more of the following deployment modes:
* Sidecar - CloudWatch Agent works at the Task level. Reasons for choosing this mode include but are not restricted to:
  * You expect a dedicated CloudWatch Agent for your Task.
* Service - CloudWatch Agent works at the Cluster level. Reasons for choosing this mode include but are not restricted to:
  * You expect CloudWatch Agent to serve for all Tasks within a Cluster.
  * You expect an centralized endpoint within a cluster for your application to access.
  * You expect an application outside the cluster to be able to access to CloudWatch Agent.
* DaemonService - CloudWatch Agent works at the Container Instance level (for Amazon ECS clusters with an EC2 launch type). Reasons for choosing this mode include but are not restricted to:
  * You expect CloudWatch Agent to serve for all Tasks within a Container Instance.
  * You expect CloudWatch Agent to collect metrics for each Container Instance.
  * You expect CloudWatch Agent to be scaled according to the number of Container Instance automatically.

You can choose the mode that is most suitable for you.

The following subfolders contain example Amazon ECS task definition files based on the deployment modes. You can refer to [register-task-definition](https://docs.aws.amazon.com/cli/latest/reference/ecs/register-task-definition.html) to register ECS task definition,
[create-task-set](https://docs.aws.amazon.com/cli/latest/reference/ecs/create-task-set.html) to create task or [create-service](https://docs.aws.amazon.com/cli/latest/reference/ecs/create-service.html) to create service.

Regarding to creating a service with a centralized endpoint in your cluster. Please refer to [Creating a Service](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-service.html)

* [sidecar](sidecar)
* [daemon-service](daemon-service)
