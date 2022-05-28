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
  default = "89d1912284dd8e60c5cd10fdddc8e12278d2eecb"
}

variable "github_repo" {
  type    = string
  default = "https://github.com/khanhntd/amazon-cloudwatch-agent"
}

variable "install_package_source" {
  default = "s3://test-bucket-2-3-4/amazon-cloudwatch-agent.msi" # Download MSI from S3
}