#Create a unique testing_id for each test
resource "random_id" "testing_id" {
  byte_length = 8
}

#####################################################################
# Generate EC2 Key Pair for log in access to EC2
#####################################################################

resource "tls_private_key" "ssh_key" {
  count     = var.ssh_key_name == "" ? 1 : 0
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "aws_ssh_key" {
  count      = var.ssh_key_name == "" ? 1 : 0
  key_name   = "ec2-key-pair-${random_id.testing_id.hex}"
  public_key = tls_private_key.ssh_key[0].public_key_openssh
}

locals {
  ssh_key_name        = var.ssh_key_name != "" ? var.ssh_key_name : aws_key_pair.aws_ssh_key[0].key_name
  private_key_content = var.ssh_key_name != "" ? var.ssh_key_value : tls_private_key.ssh_key[0].private_key_pem
}

#####################################################################
# Generate EC2 Instance and execute test commands
#####################################################################

resource "aws_instance" "cwagent" {
  ami                         = data.aws_ami.latest.id
  instance_type               = var.ec2_instance_type
  key_name                    = local.ssh_key_name
  iam_instance_profile        = aws_iam_instance_profile.cwagent_instance_profile.name
  vpc_security_group_ids      = [aws_security_group.ec2_security_group.id]
  associate_public_ip_address = true
  get_password_data           = true
  tags = {
    Name = "cwagent-integ-test-ec2-${var.test_name}-${random_id.testing_id.hex}"
  }
}

resource "null_resource" "integration_test" {
  depends_on = [aws_instance.cwagent]
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
      "echo run tests with the tag integration, one at a time, and verbose",
      "echo run sanity test && go test ./integration/test/sanity -p 1 -v --tags=integration",
    ]

    connection {
      type            = "ssh"
      user            = "Administrator"
      private_key     = local.private_key_content
      password        = rsadecrypt(aws_instance.cwagent.password_data, local.private_key_content)
      host            = aws_instance.cwagent.public_ip
      target_platform = "windows"
    }
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
