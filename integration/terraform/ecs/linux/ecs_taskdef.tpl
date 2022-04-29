[
  {
    "name": "cloudwatch_agent",
    "image": "${cwagent_image}",
    "essential": true,
    "secrets": [
      {
        "name": "CW_CONFIG_CONTENT",
        "valueFrom": "${cwagent_ssm_parameter_arn}"
      },
      {
        "name": "PROMETHEUS_CONFIG_CONTENT",
        "valueFrom": "${prometheus_ssm_parameter_arn}"
      }
    ],
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-region": "${region}",
        "awslogs-stream-prefix": "${testing_id}",
        "awslogs-group": "${log_group}"
      }
    },
    "cpu": 1,
    "mountPoints": [],
    "memory": 2048,
    "volumesFrom": []
  }
]
