variable "region" {
  type    = string
  default = "us-west-2"
}

variable "ec2_instance_type" {
  type    = string
  default = "t3a.xlarge"
}

variable "iam_instance_profile" {
  type    = string
  default = "CloudWatchAgentServerRole"
}

variable "ami" {
  type    = string
  default = "cloudwatch-agent-integration-test-win-2022*"
}

variable "github_sha" {
  type    = string
  default = "64b54f56d2d6eee016beb934b836b2d7ff8e1275"
}

variable "github_repo" {
  type    = string
  default = "https://github.com/khanhntd/amazon-cloudwatch-agent"
}

variable "ssh_key_name" {
  type = string
  default = ""
}

variable "install_package_source" {
  default = "s3://test-bucket-2-3-4/amazon-cloudwatch-agent.msi" # Download MSI from S3
}
