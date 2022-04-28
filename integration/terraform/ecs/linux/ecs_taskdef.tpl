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
        "awslogs-group": "/ecs/ecs-cwagent-prometheus",
        "awslogs-region": "${region}",
        "awslogs-stream-prefix": "ecs-FARGATE-awsvpc/cloudwatch-agent-prometheus/df194e312d944f6fbc478e13797af18f",
        "awslogs-create-group": "true"
      }
    }
  }
]