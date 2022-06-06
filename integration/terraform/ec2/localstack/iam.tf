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
      "s3:*"
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