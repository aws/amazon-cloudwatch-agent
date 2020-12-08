# CloudWatch Agent Dockerfiles

- [Dockerfile](Dockerfile) builds from the [latest release published on s3](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Agent-commandline-fleet.html)
- [locadeb](localdeb/Dockerfile) builds from a local deb file
- [source](source/Dockerfile) builds from source code, you can execute `make dockerized-build` at project root.