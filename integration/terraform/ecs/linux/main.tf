#Create a unique testing_id for each test
resource "random_id" "testing_id" {
  byte_length = 8
}

resource "aws_ecs_cluster" "cluster" {
  name = "CWAgent-Integ-Test-Cluster-${random_id.testing_id.hex}"
}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnet_ids" "default" {
  vpc_id = data.aws_vpc.default.id
}

resource "aws_cloudwatch_log_group" "log_group" {
  name = "CWAgent-Integ-Test-LG-${random_id.testing_id.hex}"
}

resource "aws_security_group" "ecs_tasks" {
  name = "${var.prefix}-tasks-sg"
  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
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
}

resource "aws_ecs_task_definition" "service" {
  family                   = "${var.prefix}-task-family"
  network_mode             = "awsvpc"
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  cpu                      = 256
  memory                   = 2048
  requires_compatibilities = ["FARGATE"]
  container_definitions = templatefile("./app.json.tpl", {
    cwagent_image     = var.cwagent_image
    ssm_parameter_arn = aws_ssm_parameter.cwagent_config.name
    log_group         = aws_cloudwatch_log_group.log_group.name
    testing_id        = random_id.testing_id.hex
    region            = var.region
  })
  depends_on = [aws_cloudwatch_log_group.log_group, aws_iam_role.ecs_task_role, aws_iam_role.ecs_task_execution_role]
}

resource "aws_ecs_service" "staging" {
  name            = "${var.prefix}-service"
  cluster         = aws_ecs_cluster.cluster.id
  task_definition = aws_ecs_task_definition.service.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    security_groups  = [aws_security_group.ecs_tasks.id]
    subnets          = data.aws_subnet_ids.default.ids
    assign_public_ip = true
  }

  depends_on = [aws_iam_role_policy_attachment.ecs_task_execution_role]
}

