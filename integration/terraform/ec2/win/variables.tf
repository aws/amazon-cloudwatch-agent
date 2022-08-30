variable "region" {
  type    = string
  default = "us-west-2"
}

variable "ec2_instance_type" {
  type    = string
  default = "t3a.xlarge"
}

variable "ami" {
  type    = string
  default = "cloudwatch-agent-integration-test-win-2022*"
}

variable "github_sha" {
  type    = string
  default = "3612edefcfded5c31b4f94371bbb4e9ebaee4284"
}

variable "github_repo" {
  type    = string
  default = "https://github.com/khanhntd/amazon-cloudwatch-agent.git"
}

variable "ssh_key_name" {
  type    = string
  default = ""
}

variable "ssh_key_value" {
  type    = string
  default = ""
}

variable "s3_bucket" {
  type    = string
  default = "integration-test-cwagent"
}

variable "test_name" {
  type    = string
  default = "windows-2022"
}
