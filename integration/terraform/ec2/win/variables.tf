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
  default = "aee2f5c9b1b0a7a840b441da37a63ede7506a343"
}

variable "github_repo" {
  type    = string
  default = "https://github.com/aws/amazon-cloudwatch-agent"
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
  default = ""
}

variable "test_name" {
  type    = string
  default = "windows-2022"
}

variable "test_dir" {
  type    = string
  default = ""
}
