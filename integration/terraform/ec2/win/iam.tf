resource "aws_iam_instance_profile" "test_profile" {
  name = "test_profile"
  role = aws_iam_role.ecs_task_role.name
}

resource "aws_iam_role" "ecs_task_role" {
  name = "cwagent-integ-test-task-role-${random_id.testing_id.hex}"

  assume_role_policy = <<EOF
{
 "Version": "2012-10-17",
 "Statement": [
   {
     "Action": "sts:AssumeRole",
     "Principal": {
       "Service": "ecs-tasks.amazonaws.com"
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
      "s3:ListBucket"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "service_discovery_policy" {
  name   = "service_discovery_policy-${random_id.testing_id.hex}"
  policy = data.aws_iam_policy_document.user-managed-policy-document.json
}

resource "aws_iam_role_policy_attachment" "service_discovery_task" {
  role       = aws_iam_role.ecs_task_role.name
  policy_arn = aws_iam_policy.service_discovery_policy.arn
}