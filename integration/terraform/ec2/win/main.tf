#Create a unique testing_id for each test
resource "random_id" "testing_id" {
  byte_length = 8
}

#####################################################################
# Generate EC2 Key Pair for log in access to EC2
#####################################################################

resource "tls_private_key" "ssh_key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "aws_ssh_key" {
  key_name   = "ec2-key-pair-${random_id.testing_id.hex}"
  public_key = "${tls_private_key.ssh_key.public_key_openssh}"
}

#####################################################################
# Generate EC2 Instance and execute test commands
#####################################################################

resource "aws_instance" "integration-test" {
  ami                    = data.aws_ami.latest.id
  instance_type          = var.ec2_instance_type
  key_name               = aws_key_pair.aws_ssh_key.key_name
  iam_instance_profile   = aws_iam_role.ecs_task_role.name
  vpc_security_group_ids = [aws_security_group.ecs_security_group.id]
  get_password_data = true

  provisioner "remote-exec" {
    # @TODO when @ZhenyuTan-amz adds windows tests add "make integration-test"
    # @TODO add export for AWS region from tf vars to make sure runner can use AWS SDK
    inline = [
      "echo clone and install agent",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "aws s3 cp ${var.install_package_source} .",
      "msiexec /i amazon-cloudwatch-agent.msi",
    ]

    connection {
      type     = "ssh"
      user     = "Administrator"
      private_key = aws_key_pair.aws_ssh_key.key_name
      password = rsadecrypt(self.password_data, aws_key_pair.aws_ssh_key.key_name)
      host     = self.public_dns
      target_platform = "windows"
    }
  }

  tags = {
    Name = "cwagent-integ-test-${random_id.testing_id.hex}"
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
