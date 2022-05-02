variable "region" {
  type    = string
  default = "us-west-2"
}

variable "cwagent_image_repo" {
  type    = string
  default = "167129616597.dkr.ecr.us-west-2.amazonaws.com/cwagent-testing"
}

variable "cwagent_image_tag" {
  type = string
  default = "fargate_test"
}

variable "test_dir" {
  type    = string
  default = "../../../test/ecs/ecs_metadata"
}