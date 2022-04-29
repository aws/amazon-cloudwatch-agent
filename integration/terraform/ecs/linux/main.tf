#Create a unique testing_id for each test
resource "random_id" "testing_id" {
  byte_length = 8
}

resource "aws_ecs_cluster" "cluster" {
  name = "cwagent-integ-test-cluster-${random_id.testing_id.hex}"
}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnet_ids" "default" {
  vpc_id = data.aws_vpc.default.id
}

resource "aws_cloudwatch_log_group" "log_group" {
  name = "cwagent-integ-test-log-group"
}

resource "aws_security_group" "ecs_security_group" {
  name = "cwagent-sg"
  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

data "template_file" "cwagent_config" {
  template = file("./default_amazon-cloudwatch-agent.json")
  vars = {
  }
}

resource "aws_ssm_parameter" "cwagent_config" {
  name  = "cwagent-config"
  type  = "String"
  value = data.template_file.cwagent_config.rendered
}

data "template_file" "task_def" {
  template = file("./ecs_taskdef.tpl")
  vars = {
    region            = var.region
    ssm_parameter_arn = aws_ssm_parameter.cwagent_config.name
    cwagent_image     = var.cwagent_image
    log_group         = aws_cloudwatch_log_group.log_group.name
    testing_id        = random_id.testing_id.hex
  }
}

resource "aws_ecs_task_definition" "task_definition" {
  family                   = "cwagent-task-family-${random_id.testing_id.hex}"
  network_mode             = "awsvpc"
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  cpu                      = 256
  memory                   = 2048
  requires_compatibilities = ["FARGATE"]
  container_definitions    = data.template_file.task_def.rendered
  depends_on               = [aws_cloudwatch_log_group.log_group, aws_iam_role.ecs_task_role, aws_iam_role.ecs_task_execution_role]
}

resource "aws_ecs_service" "service" {
  name            = "cwagent-service-${random_id.testing_id.hex}"
  cluster         = aws_ecs_cluster.cluster.id
  task_definition = aws_ecs_task_definition.task_definition.arn
  desired_count   = 1
  launch_type     = "FARGATE"
  wait_for_steady_state = true

  network_configuration {
    security_groups  = [aws_security_group.ecs_security_group.id]
    subnets          = data.aws_subnet_ids.default.ids
    assign_public_ip = false
  }

  depends_on = [aws_iam_role_policy_attachment.ecs_task_execution_role]
}

resource "null_resource" "push" {
  provisioner "local-exec" {
    command     = "echo command"
    interpreter = ["bash", "-c"]
  }
  depends_on = [aws_ecs_service.service]
}