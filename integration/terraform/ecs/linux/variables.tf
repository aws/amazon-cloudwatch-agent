variable "region" {
  type = string
  default = "us-west-2"
}

variable "name" {
  type = string
  default = "testcluster"
}

variable "cwagent_image" {
  type = string
  default = "167129616597.dkr.ecr.us-west-2.amazonaws.com/cwagent-testing:fargate"
}