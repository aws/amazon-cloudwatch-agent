variable "ec2_instance_type" {
  type = string
  default = "t3a.xlarge"
}

variable "key_name" {
  type = string
  default = "cwagent-integ-test-key"
}

variable "iam_instance_profile" {
  type = string
  default = "CloudWatchAgentServerRole"
}

variable "vpc_security_group_ids" {
  type = list(string)
  default = ["sg-013585129c1f92bf0"]
}

variable "region" {
  type = string
  default = "us-west-2"
}

variable "ami" {
  type = string
  default = "cloudwatch-agent-integration-test-win-2022*"
}

variable "ssh_key" {
  type = string
  default = ""
}

variable "github_sha" {
  type = string
  default = ""
}

variable "github_repo" {
  type = string
  default = ""
}

variable "test_name" {
  type = string
  default = ""
}

variable "s3_bucket" {
  type = string
  default = ""
}