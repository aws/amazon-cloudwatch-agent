resource "aws_instance" "integration-test" {
  ami                    = data.aws_ami.latest.id
  instance_type          = var.ec2_instance_type
  key_name               = var.key_name
  iam_instance_profile   = var.iam_instance_profile
  vpc_security_group_ids = var.vpc_security_group_ids
  get_password_data = true
  provisioner "remote-exec" {
    # @TODO when @ZhenyuTan-amz adds windows tests add "make integration-test"
    # @TODO add export for AWS region from tf vars to make sure runner can use AWS SDK
    inline = [
      "echo clone and install agent",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "aws s3 cp s3://${var.s3_bucket}/integration-test/packaging/${var.github_sha}/amazon-cloudwatch-agent.msi .",
      "msiexec /i amazon-cloudwatch-agent.msi",
    ]
    connection {
      type     = "ssh"
      user     = "Administrator"
      private_key = var.ssh_key
      password = rsadecrypt(self.password_data, var.ssh_key)
      host     = self.public_dns
      target_platform = "windows"
    }
  }
  tags = {
    Name = var.test_name
  }
}

data "aws_ami" "latest" {
  most_recent = true
  owners      = ["self", "506463145083"]

  filter {
    name   = "name"
    values = [var.ami]
  }
}