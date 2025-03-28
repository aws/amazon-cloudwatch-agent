# Build the binary
ARG CERT_IMAGE=ubuntu:latest

# Install cert and binaries
FROM $CERT_IMAGE as cert

# Need to repeat the ARG after each FROM
ARG TARGETARCH
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/etc
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/var
RUN apt-get update &&  \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY build/bin/linux_${TARGETARCH}/ /opt/aws/amazon-cloudwatch-agent/bin

FROM scratch

COPY --from=cert /tmp /tmp
COPY --from=cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=cert /opt/aws/amazon-cloudwatch-agent /opt/aws/amazon-cloudwatch-agent

ENV RUN_IN_CONTAINER="True"
ENTRYPOINT ["/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent"]
