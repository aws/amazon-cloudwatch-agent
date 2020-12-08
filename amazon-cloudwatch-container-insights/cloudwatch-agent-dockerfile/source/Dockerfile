# Build the binary
FROM golang:latest as builder

RUN mkdir -p /go/src/github.com/aws/amazon-cloudwatch-agent/
WORKDIR /go/src/github.com/aws/amazon-cloudwatch-agent/

ARG GO111MODULE="on"
ENV GO111MODULE=${GO111MODULE}

COPY . /go/src/github.com/aws/amazon-cloudwatch-agent/
RUN make build && make package-deb

# Install cert and binaries
FROM ubuntu:latest as cert

# NOTE: This arg will be populated by docker buildx
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETARCH
RUN apt-get update &&  \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /go/src/github.com/aws/amazon-cloudwatch-agent/build/bin/linux/ /tmp/deb
RUN dpkg -i -E /tmp/deb/${TARGETARCH:-$(dpkg --print-architecture)}/amazon-cloudwatch-agent.deb

FROM scratch

COPY --from=cert /tmp /tmp
COPY --from=cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=cert /opt/aws/amazon-cloudwatch-agent /opt/aws/amazon-cloudwatch-agent

ENV RUN_IN_CONTAINER="True"
ENTRYPOINT ["/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent"]
