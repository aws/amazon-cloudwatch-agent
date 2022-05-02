variable "region" {
  type    = string
  default = "us-east-2"
}

variable "cwagent_image" {
  type    = string
  default = "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:latest"
}

variable "test_dir" {
  type    = string
  default = "../../../test/ecs/ecs_metadata/"
}