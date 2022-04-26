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
  default = "cloudwatch-agent-integration-test-ubuntu*"
}

variable "ssh_key" {
  type = string
  default = ""
}

variable "user" {
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

variable "install_agent" {
  description = "command of package to install ex dpkg -i -E ./amazon-cloudwatch-agent.deb"
  type = string
  default = ""
}

variable "ca_cert_path" {
  type = string
  default = ""
}

variable "arc" {
  type = string
  default = ""
}

variable "binary_name" {
  type = string
  default = ""
}

variable "local_stack_host_name" {
  type = string
  default = "localhost.localstack.cloud"
}

variable "s3_bucket" {
  type = string
  default = ""
}

variable "test_name" {
  type = string
  default = ""
}

variable "test_dir" {
  type = string
  default = ""
}