ARG BUILD_IMAGE=public.ecr.aws/amazonlinux/amazonlinux:latest
FROM $BUILD_IMAGE AS build

ARG TARGETARCH

RUN yum update -y &&  \
    yum install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY ${TARGETARCH}/amazon-cloudwatch-agent.rpm /tmp/amazon-cloudwatch-agent.rpm

RUN rpm -i /tmp/amazon-cloudwatch-agent.rpm && \
    rm -rf /tmp/* && \
    rm -rf /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-config-wizard && \
    rm -rf /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl && \
    rm -rf /opt/aws/amazon-cloudwatch-agent/bin/config-downloader

FROM scratch

# Need to create /tmp or plugin files such as prometheus is missing
# Must use copy because scratch does not contain shell so mkdir won't work
COPY --from=build /tmp /tmp
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-bundle.trust.crt
COPY --from=build /opt/aws/amazon-cloudwatch-agent /opt/aws/amazon-cloudwatch-agent

ENV RUN_IN_CONTAINER="True"
ENTRYPOINT ["/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent"]