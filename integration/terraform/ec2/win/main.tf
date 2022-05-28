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
  count     = var.ssh_key_name == "" ? 1 : 0
  key_name   = "ec2-key-pair-${random_id.testing_id.hex}"
  public_key = tls_private_key.ssh_key[0].public_key_openssh
}

#####################################################################
# Generate EC2 Instance and execute test commands
#####################################################################

resource "aws_instance" "cwagent" {
  ami                         = data.aws_ami.latest.id
  instance_type               = var.ec2_instance_type
  key_name                    = aws_key_pair.aws_ssh_key[0].key_name
  iam_instance_profile        = aws_iam_instance_profile.cwagent_instance_profile.name
  vpc_security_group_ids      = [aws_security_group.ecs_security_group.id]
  associate_public_ip_address = true
  get_password_data           = true
  tags = {
    Name = "cwagent-integ-test-${random_id.testing_id.hex}"
  }
}

resource "null_resource" "integration_test" {
  depends_on = [aws_instance.cwagent]
  provisioner "remote-exec" {
    inline = [
      "echo clone and install agent",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "msiexec /i https://test-bucket-2-3-4.s3.us-west-2.amazonaws.com/amazon-cloudwatch-agent.msi",
      "echo run tests with the tag integration, one at a time, and verbose",
      "echo run sanity test && go test ./integration/test/sanity -p 1 -v --tags=integration",
    ]

    connection {
      type            = "ssh"
      user            = "Administrator"
      private_key     = tls_private_key.ssh_key[0].private_key_pem
      password        = rsadecrypt(aws_instance.cwagent.password_data, tls_private_key.ssh_key[0].private_key_pem)
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
