variable "ec2_instance_type" {
  type    = string
  default = "t3a.xlarge"
}

variable "ssh_key_name" {
  type    = string
  default = "cwagent-integ-test-key"
}

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "ssh_key_value" {
  type    = string
  default = ""
}

variable "github_sha" {
  type    = string
  default = ""
}

variable "github_repo" {
  type    = string
  default = ""
}

variable "s3_bucket" {
  type    = string
  default = ""
}

output "public_dns" {
  description = "The public DNS name assigned to the instance. For EC2-VPC, this is only available if you've enabled DNS hostnames for your VPC"
  value       = aws_instance.integration-test.public_dns
}