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
  user_data                   = <<EOF
<powershell>
Write-Output "Install OpenSSH and Firewalls which allows port 22 for connection"
Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0

Start-Service sshd
Set-Service -Name sshd -StartupType 'Automatic'

[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
Set-ExecutionPolicy Bypass -Scope Process -Force; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))

choco install git --confirm
choco install go --confirm

msiexec.exe /i https://awscli.amazonaws.com/AWSCLIV2.msi  /norestart /qb-
</powershell>
EOF

  tags = {
    Name = "cwagent-integ-test-ec2-${var.test_name}-${random_id.testing_id.hex}"
  }
}

resource "null_resource" "integration_test" {
  depends_on = [aws_instance.cwagent]
  # Install software
  provisioner "remote-exec" {
    inline = [
      "refreshenv",
      "set AWS_REGION=${var.region}",
      "echo clone and install agent",
      "git clone ${var.github_repo}",
      "cd amazon-cloudwatch-agent",
      "git reset --hard ${var.github_sha}",
      "aws s3 cp s3://${var.s3_bucket}/integration-test/packaging/${var.github_sha}/amazon-cloudwatch-agent.msi .",
      "msiexec /i amazon-cloudwatch-agent.msi",
      "echo run tests with the tag integration, one at a time, and verbose",
      "echo run sanity test && go test ./integration/test/sanity -p 1 -v --tags=integration",
      "cd ./integration/test/nvidia_gpu",
      "go test . -p 1 -timeout 30m -v --tags=integration "
    ]

    connection {
      type            = "ssh"
      user            = "Administrator"
      password        = rsadecrypt(aws_instance.cwagent.password_data, local.private_key_content)
      host            = aws_instance.cwagent.public_ip
      target_platform = "windows"
      timeout         = "6m"
    }
  }
}

data "aws_ami" "latest" {
  most_recent = true
  // @Todo: Add back when nvidia_gpu pipeline has been able to produced the AMI
  #owners      = ["self", "506463145083","aws-marketplace"]

  filter {
    name   = "name"
    values = [var.ami]
  }
}
