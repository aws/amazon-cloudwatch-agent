resource "aws_instance" "integration-test" {
  ami                    = data.aws_ami.latest.id
  instance_type          = var.ec2_instance_type
  key_name               = var.key_name
  iam_instance_profile   = var.iam_instance_profile
  vpc_security_group_ids = var.vpc_security_group_ids
  provisioner "remote-exec" {
    inline = [
      "cloud-init status --wait",
      "echo clone and install agent",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "aws s3 cp s3://${var.s3_bucket}/integration-test/binary/${var.github_sha}/linux/${var.arc}/${var.binary_name} .",
      "sudo ${var.install_agent}",
      "echo get ssl pem for localstack and export local stack host name",
      "cd ~/amazon-cloudwatch-agent/integration/localstack/ls_tmp",
      "aws s3 cp s3://${var.s3_bucket}/integration-test/ls_tmp/${var.github_sha} . --recursive",
      "cat ${var.ca_cert_path} > original.pem",
      "cat original.pem snakeoil.pem > combine.pem",
      "sudo cp original.pem /opt/aws/amazon-cloudwatch-agent/original.pem",
      "sudo cp combine.pem /opt/aws/amazon-cloudwatch-agent/combine.pem",
      "export LOCAL_STACK_HOST_NAME=${var.local_stack_host_name}",
      "export AWS_REGION=${var.region}",
      "echo run tests with the tag integration, one at a time, and verbose",
      "cd ~/amazon-cloudwatch-agent",
      "echo run sanity test && go test ./integration/test/sanity -p 1 -v --tags=integration",
      "go test ${var.test_dir} -p 1 -v --tags=integration"
    ]
    connection {
      type        = "ssh"
      user        = var.user
      private_key = var.ssh_key
      host        = self.public_dns
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