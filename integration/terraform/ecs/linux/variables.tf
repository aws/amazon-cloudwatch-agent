variable "region" {
  type    = string
  default = "us-west-2"
}

variable "cwagent_image" {
  type    = string
  default = "167129616597.dkr.ecr.us-west-2.amazonaws.com/cwagent-testing:fargate_test"
}

variable "cwagent_config" {
  type = string
  default = "./default_amazon_cloudwatch_agent.json"
}