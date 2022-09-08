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
  default = "Windows_Server-2019-English-Deep-Learning*"
}

variable "github_sha" {
  type    = string
  default = "4cefc9f0e9b411c6765c5122877c136dbb23588d"
}

variable "github_repo" {
  type    = string
  default = "https://github.com/aws/amazon-cloudwatch-agent.git"
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

variable "test_dir" {
  type    = string
  default = "./integration/test/nvidia_gpu"
}
