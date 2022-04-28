[
  {
    "name": "cloudwatch-agent",
    "image": "${cwagent_image}",
    "cpu": 10,
    "memory": 256,
    "secrets": [
      {
        "name": "CW_CONFIG_CONTENT",
        "valueFrom": "${ssm_parameter_arn}"
      }
    ],
    "essential": true,
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group": "ecs",
        "awslogs-region": "${region}",
        "awslogs-stream-prefix": "ecs-FARGATE-awsvpc",
        "awslogs-create-group": "true"
      }
    }
  }
]