#####################################################################
# Ensure there is unique testing_id for each test
#####################################################################
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

resource "aws_instance" "integration-test" {
  ami                    = data.aws_ami.latest.id
  instance_type          = var.ec2_instance_type
  key_name               = local.ssh_key_name
  iam_instance_profile   = aws_iam_instance_profile.cwagent_instance_profile.name
  vpc_security_group_ids = [aws_security_group.ec2_security_group.id]
  provisioner "remote-exec" {
    inline = [
      "cloud-init status --wait",
      "clone the agent and start the localstack",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "echo set up ssl pem for localstack, then start localstack",
      "cd ~/amazon-cloudwatch-agent/integration/localstack/ls_tmp",
      "openssl req -new -x509 -newkey rsa:2048 -sha256 -nodes -out snakeoil.pem -keyout snakeoil.key -config snakeoil.conf",
      "cat snakeoil.key snakeoil.pem > server.test.pem",
      "cat snakeoil.key > server.test.pem.key",
      "cat snakeoil.pem > server.test.pem.crt",
      "cd ~/amazon-cloudwatch-agent/integration/localstack",
      "docker-compose up -d --force-recreate",
      "aws s3 cp ls_tmp s3://${var.s3_bucket}/integration-test/ls_tmp/${var.github_sha} --recursive"
    ]
    connection {
      type        = "ssh"
      user        = "ubuntu"
      private_key = local.private_key_content
      host        = self.public_dns
    }
  }

  tags = {
    Name = "LocalStackIntegrationTestInstance"
  }
}

data "aws_ami" "latest" {
  most_recent = true
  owners      = ["self", "506463145083"]

  filter {
    name   = "name"
    values = ["cloudwatch-agent-integration-test-ubuntu*"]
  }
}
