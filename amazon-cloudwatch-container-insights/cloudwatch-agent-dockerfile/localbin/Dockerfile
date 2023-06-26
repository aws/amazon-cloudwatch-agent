ARG BUILD_IMAGE=ubuntu:latest

# Install cert and binaries
FROM $BUILD_IMAGE as build

# Need to repeat the ARG after each FROM
ARG TARGETARCH

RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/etc
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/var
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/bin
COPY tmp/${TARGETARCH} /opt/aws/amazon-cloudwatch-agent/bin
RUN apt-get update &&  \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

FROM scratch

COPY --from=build /tmp /tmp
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /opt/aws/amazon-cloudwatch-agent /opt/aws/amazon-cloudwatch-agent

ENV RUN_IN_CONTAINER="True"
ENTRYPOINT ["/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent"]
