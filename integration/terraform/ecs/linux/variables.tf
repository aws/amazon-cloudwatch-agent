variable "region" {
  type    = string
  default = "us-west-2"
}

variable "cwagent_image_repo" {
  type    = string
  default = "public.ecr.aws/cloudwatch-agent/cloudwatch-agent"
}

variable "cwagent_image_tag" {
  type    = string
  default = "latest"
}

variable "test_dir" {
  type    = string
  default = "./integration/test/ecs/ecs_metadata"
}