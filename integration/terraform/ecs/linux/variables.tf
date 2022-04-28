variable "region" {
  type    = string
  default = "us-west-2"
}

variable "name" {
  type    = string
  default = "testcluster"
}

variable "cwagent_image" {
  type    = string
  default = "167129616597.dkr.ecr.us-west-2.amazonaws.com/cwagent-testing:fargate"
}

variable "prefix" {
  description = "prefix prepended to names of all resources created"
  default     = "aws-terraform-test"
}

variable "port" {
  description = "port the container exposes, that the load balancer should forward port 80 to"
  default     = "4000"
}

variable "source_path" {
  description = "source path for project"
  default     = "./project"
}

variable "envvars" {
  type        = map(string)
  description = "variables to set in the environment of the container"
  default = {
  }
}