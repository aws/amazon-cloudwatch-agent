variable "region" {
  type    = string
  default = "us-east-2"
}

variable "cwagent_image" {
  type    = string
  default = "167129616597.dkr.ecr.us-east-2.amazonaws.com/cwagent-testing:test_again"
}

variable "cwagent_config" {
  type    = string
  default = "./default_amazon_cloudwatch_agent.json"
}

variable "ecs_taskdef" {
  type    = string
  default = "./default_ecs_taskdef.tpl"
}

variable "ecs_extra_apps" {
  type    = string
  default = ""
}