resource "aws_ecs_cluster" "main" {
  name = "${var.name}-cluster"

}

resource "aws_ecs_task_definition" "main" {
  family                = "taskdef-${var.name}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  container_definitions    = jsonencode([{
    name              : "cwagent",
    image             : "167129616597.dkr.ecr.us-west-2.amazonaws.com/cwagent-testing:fargate",
    essential         : true,
    secrets           : [{
      name: "CW_CONFIG_CONTENT",
      valueFrom: "arn:aws:ssm:us-west-2:167129616597:parameter/AmazonCloudWatch-CWAgentConfig-CWAgentFargateTestUpdate-FARGATE-awsvpc",
    }]
  }])
}

resource "aws_ecs_service" "main" {
 name                               = "${var.name}-service"
 cluster                            = aws_ecs_cluster.main.id
 task_definition                    = aws_ecs_task_definition.main.arn
 desired_count                      = 2
 deployment_minimum_healthy_percent = 50
 deployment_maximum_percent         = 200
 launch_type                        = "FARGATE"
 scheduling_strategy                = "REPLICA"

 lifecycle {
   ignore_changes = [task_definition, desired_count]
 }
 network_configuration {
    security_groups  = ["sg-038da11275feb85cd"]
    subnets          = ["subnet-0f6a1cbcfde2da248"]
    assign_public_ip = false
  }
}

