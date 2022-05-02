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

data "aws_iam_policy_document" "ecs_task_execution_role" {
  version = "2012-10-17"
  statement {
    sid     = ""
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task_execution_role" {
  name = "cwagent-integ-test-task-execution-role-${random_id.testing_id.hex}"

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
      "ecs:DescribeTasks",
      "ecs:ListTasks",
      "ecs:DescribeContainerInstances",
      "ecs:DescribeServices",
      "ecs:ListServices",
      "ec2:DescribeInstances",
      "ecs:DescribeTaskDefinition"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "service_discovery_policy" {
  name   = "service_discovery_policy-${random_id.testing_id.hex}"
  policy = data.aws_iam_policy_document.user-managed-policy-document.json

}

resource "aws_iam_role_policy_attachment" "ecs_task_execution_role" {
  role       = aws_iam_role.ecs_task_execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy_attachment" "ssm_task_execution_role" {
  role       = aws_iam_role.ecs_task_execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMReadOnlyAccess"
}

resource "aws_iam_role_policy_attachment" "agent_task" {
  role       = aws_iam_role.ecs_task_role.name
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
}

resource "aws_iam_role_policy_attachment" "service_discovery_task" {
  role       = aws_iam_role.ecs_task_role.name
  policy_arn = aws_iam_policy.service_discovery_policy.arn
}
