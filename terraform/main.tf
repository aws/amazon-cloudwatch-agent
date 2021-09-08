resource "aws_instance" "integration-test" {
  ami           = var.ami
  instance_type = var.ec2_instance_type
  key_name = var.key_name
  iam_instance_profile = var.iam_instance_profile
  vpc_security_group_ids = var.vpc_security_group_ids
  provisioner "remote-exec" {
    inline = [
      "cloud-init status --wait",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "make clean build ${var.package}",
      "cd build/bin/linux/amd64",
      "sudo ${var.install_agent}"
    ]
    connection {
      type = "ssh"
      user = var.user
      private_key = var.ssh_key
      host = self.public_dns
    }
  }
}