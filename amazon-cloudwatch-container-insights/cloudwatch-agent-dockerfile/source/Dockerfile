# Build the binary
ARG GO_IMAGE=golang:latest
ARG CERT_IMAGE=ubuntu:latest
FROM $GO_IMAGE as builder

# NOTE: This arg will be populated by docker buildx
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETARCH
ENV GOPROXY=direct
RUN mkdir -p /go/src/github.com/aws/amazon-cloudwatch-agent/
WORKDIR /go/src/github.com/aws/amazon-cloudwatch-agent/

ARG GO111MODULE="on"
ENV GO111MODULE=${GO111MODULE}

COPY go.mod /go/src/github.com/aws/amazon-cloudwatch-agent/
COPY go.sum /go/src/github.com/aws/amazon-cloudwatch-agent/
RUN go mod download -x

COPY . /go/src/github.com/aws/amazon-cloudwatch-agent/

RUN make build-for-docker-${TARGETARCH}

# Install cert and binaries
FROM $CERT_IMAGE as cert

# Need to repeat the ARG after each FROM
ARG TARGETARCH
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/etc
RUN mkdir -p /opt/aws/amazon-cloudwatch-agent/var
RUN apt-get update &&  \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /go/src/github.com/aws/amazon-cloudwatch-agent/build/bin/linux_${TARGETARCH}/ /opt/aws/amazon-cloudwatch-agent/bin

FROM scratch

COPY --from=cert /tmp /tmp
COPY --from=cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=cert /opt/aws/amazon-cloudwatch-agent /opt/aws/amazon-cloudwatch-agent

ENV RUN_IN_CONTAINER="True"
ENTRYPOINT ["/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent"]
