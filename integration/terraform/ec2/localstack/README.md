The localstack ec2 will bring up an instance of localstack on an ec2 instance to be used by other tests

Use the ubuntu ami

Instance assumptions
1. Use the same ami as the ubuntu integration test (if you use this ami you can ignore the other assumptions)
2. docker
3. docker-compose
4. git
5. openssl
6. aws-cli
7. CloudWatchAgentServerRole is attached
8. crontab

Tag the ec2 instance with LocalStackIntegrationTestInstance

Output will be the public dns to be used for https connection and the proper ssl pem files (uploaded to s3)
