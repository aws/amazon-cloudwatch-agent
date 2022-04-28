resource "aws_ecs_cluster" "main" {
  name = "${var.name}-cluster"

}
data "template_file" "cwagent_config" {
  template = file("./amazon-cloudwatch-agent.json")
  vars = {

  }
}

resource "aws_ssm_parameter" "cwagent_config" {
  name  = "cwagent-config"
  type  = "String"
  value = data.template_file.cwagent_config.rendered
  tier  = "Advanced" // need advanced for a long list of prometheus relabel config
}

data "template_file" "task_def" {
  template = file("./ecs_taskdef.tpl")

  vars = {
    region                         = var.region
    ssm_parameter_arn              = aws_ssm_parameter.cwagent_config.name
    cwagent_image                  = var.cwagent_image
  }
}

resource "aws_ecs_task_definition" "main" {
  family                   = "taskdef-${var.name}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  container_definitions    = data.template_file.task_def.rendered
}

resource "aws_ecs_service" "main" {
 name                               = "${var.name}-service"
 cluster                            = aws_ecs_cluster.main.id
 task_definition                    = aws_ecs_task_definition.main.arn
 desired_count                      = 1
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

