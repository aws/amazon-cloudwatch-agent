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
    "memory": 1024,
    "volumesFrom": []
  },
  {
    "name": "memcached-0",
    "image": "memcached:1.6.7",
    "essential": true,
    "portMappings": [
      {
        "protocol":"tcp",
        "containerPort": 11211
      }
    ],
    "dockerLabels": {
      "app": "memcached"
    },
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-region": "${region}",
        "awslogs-stream-prefix": "memcached-tutorial",
        "awslogs-group": "${log_group}"
      }
    },
    "cpu": 64,
    "mountPoints": [ ],
    "memory": 1024,
    "volumesFrom": [ ]
  },
  {
    "name": "memcached-exporter-0",
    "image": "prom/memcached-exporter:v0.7.0",
    "essential": true,
    "portMappings": [
      {
        "protocol":"tcp",
        "containerPort": 9150
      }
    ],
    "dockerLabels":{
      "job": "prometheus-memcached",
      "app_x": "memcached_exporter"
    },
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-region": "${region}",
        "awslogs-stream-prefix": "memcached-exporter-tutorial",
        "awslogs-group": "${log_group}"
      }
    },
    "cpu": 64,
    "mountPoints": [ ],
    "memory": 1024,
    "volumesFrom": [ ]
  }
]
