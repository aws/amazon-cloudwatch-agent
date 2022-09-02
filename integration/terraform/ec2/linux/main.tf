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

  tags = {
    Name = "cwagent-integ-test-ec2-${var.test_name}-${random_id.testing_id.hex}"
  }
}

resource "null_resource" "integration_test" {
  # Prepare Integration Test
  provisioner "remote-exec" {
    inline = [
      "echo sha ${var.github_sha}",
      "cloud-init status --wait",
      "echo clone and install agent",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "aws s3 cp s3://${var.s3_bucket}/integration-test/binary/${var.github_sha}/linux/${var.arc}/${var.binary_name} .",
      "sleep 10",
      "sudo ${var.install_agent}",
      "echo get ssl pem for localstack and export local stack host name",
      "cd ~/amazon-cloudwatch-agent/integration/localstack/ls_tmp",
      "aws s3 cp s3://${var.s3_bucket}/integration-test/ls_tmp/${var.github_sha} . --recursive",
      "cat ${var.ca_cert_path} > original.pem",
      "cat original.pem snakeoil.pem > combine.pem",
      "sudo cp original.pem /opt/aws/amazon-cloudwatch-agent/original.pem",
      "sudo cp combine.pem /opt/aws/amazon-cloudwatch-agent/combine.pem",
    ]

    connection {
      type        = "ssh"
      user        = var.user
      private_key = local.private_key_content
      host        = aws_instance.cwagent.public_ip
    }
  }

  #Run sanity check and integration test
  provisioner "remote-exec" {
    inline = [
      "echo prepare environment",
      "export LOCAL_STACK_HOST_NAME=${var.local_stack_host_name}",
      "export AWS_REGION=${var.region}",
      "export PATH=$PATH:/snap/bin:/usr/local/go/bin",
      "echo run integration test",
      "cd ~/amazon-cloudwatch-agent",
      "echo run sanity test && go test ./integration/test/sanity -p 1 -v --tags=integration",
      "export SHA=${var.github_sha}",
      "export SHA_DATE=${var.github_sha_date}",
      "export PERFORMANCE_NUMBER_OF_LOGS=${var.performance_number_of_logs}",
      "go test ${var.test_dir} -p 1 -timeout 30m -v --tags=integration "
    ]
    connection {
      type        = "ssh"
      user        = var.user
      private_key = local.private_key_content
      host        = aws_instance.cwagent.public_ip
    }
  }

  depends_on = [aws_instance.cwagent]
}

data "aws_ami" "latest" {
  most_recent = true
  owners      = ["self", "506463145083"]

  filter {
    name   = "name"
    values = [var.ami]
  }
}