# Install cert and binaries
FROM amazoncorretto:latest

# Need to repeat the ARG after each FROM
ARG TARGETARCH

RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/etc
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/var
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/bin
COPY tmp/${TARGETARCH} /opt/aws/amazon-cloudwatch-agent/bin
RUN yum update -y && yum install -y ca-certificates java

ENV RUN_IN_CONTAINER="True"
ENTRYPOINT ["/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent"]
