resource "aws_iam_instance_profile" "cwagent_instance_profile" {
  name = "cwagent-instance-profile-${random_id.testing_id.hex}"
  role = aws_iam_role.cwagent_role.name
}

resource "aws_iam_role" "cwagent_role" {
  name = "cwagent-integ-test-task-role-${random_id.testing_id.hex}"

  assume_role_policy = <<EOF
{
 "Version": "2012-10-17",
 "Statement": [
   {
     "Action": "sts:AssumeRole",
     "Principal": {
       "Service": "ec2.amazonaws.com"
     },
     "Effect": "Allow",
     "Sid": ""
   }
 ]
}
EOF
}


data "aws_iam_policy_document" "user-managed-policy-document" {
  statement {
    actions = [
      "cloudwatch:GetMetricData",
      "cloudwatch:PutMetricData",
      "cloudwatch:ListMetrics",
      "ec2:DescribeVolumes",
      "ec2:DescribeTags",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
      "logs:DescribeLogGroups",
      "logs:CreateLogStream",
      "logs:CreateLogGroup",
      "logs:DeleteLogGroup",
      "logs:DeleteLogStream",
      "logs:PutRetentionPolicy",
      "logs:GetLogEvents",
      "logs:PutLogEvents",
      "s3:GetObjectAcl",
      "s3:GetObject",
      "s3:ListBucket",
      "dynamodb:DescribeTable",
      "dynamodb:PutItem",
      "dynamodb:CreateTable"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "cwagent_server_policy" {
  name   = "cwagent-server-policy-${random_id.testing_id.hex}"
  policy = data.aws_iam_policy_document.user-managed-policy-document.json
}

resource "aws_iam_role_policy_attachment" "cwagent_server_policy_attachment" {
  role       = aws_iam_role.cwagent_role.name
  policy_arn = aws_iam_policy.cwagent_server_policy.arn
}