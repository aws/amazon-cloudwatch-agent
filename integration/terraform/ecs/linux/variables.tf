variable "region" {
  type    = string
  default = "us-east-2"
}

variable "cwagent_image" {
  type    = string
  default = "167129616597.dkr.ecr.us-east-2.amazonaws.com/cwagent-testing:test_again"
}

variable "test_dir" {
  type    = string
  default = "../../../test/ecs/ecs_metadata/"
}