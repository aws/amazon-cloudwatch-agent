variable "region" {
  type    = string
  default = "us-west-2"
}

variable "ec2_instance_type" {
  type    = string
  default = "g4dn.xlarge"
}

variable "ami" {
  type    = string
  default = "Windows_Server-2019-English-Deep-Learning-2022.07.13*"
}

variable "github_sha" {
  type    = string
  default = "8200377b50ffeb34fd37471bdfb49b58722bfcdd"
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

variable "test_dir" {
  type    = string
  default = ""
}
