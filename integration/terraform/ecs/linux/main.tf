


## create task def
data "template_file" "task_def" {
  template = file(local.ecs_taskdef_path)

  vars = {
    region                         = var.region
    aoc_image                      = module.common.aoc_image
    data_emitter_image             = local.sample_app_image
    testing_id                     = module.common.testing_id
    otel_service_namespace         = module.common.otel_service_namespace
    otel_service_name              = module.common.otel_service_name
    ssm_parameter_arn              = aws_ssm_parameter.otconfig.name
    sample_app_container_name      = module.common.sample_app_container_name
    sample_app_listen_address      = "${module.common.sample_app_listen_address_ip}:${module.common.sample_app_listen_address_port}"
    sample_app_listen_address_host = module.common.sample_app_listen_address_ip
    sample_app_listen_port         = module.common.sample_app_listen_address_port
    udp_port                       = module.common.udp_port
    grpc_port                      = module.common.grpc_port
    http_port                      = module.common.http_port

    mocked_server_image = local.mocked_server_image
  }
}

resource "aws_ecs_task_definition" "main" {
  count                    = 1
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  container_definitions = jsonencode([{
   name        = "${var.name}-container-${var.environment}"
   image       = "${var.container_image}:latest"
   essential   = true
   environment = var.container_environment
   portMappings = [{
     protocol      = "tcp"
     containerPort = var.container_port
     hostPort      = var.container_port
   }]
}