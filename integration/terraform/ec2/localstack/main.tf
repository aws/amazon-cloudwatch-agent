resource "aws_instance" "integration-test" {
  ami                    = data.aws_ami.latest.id
  instance_type          = var.ec2_instance_type
  key_name               = var.key_name
  iam_instance_profile   = var.iam_instance_profile
  vpc_security_group_ids = var.vpc_security_group_ids
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
      private_key = var.ssh_key
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
