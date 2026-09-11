#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent - install (Linux)
#
# Downloads and installs the CloudWatch Agent package, then configures and
# starts it with the default OpenTelemetry (OTLP) configuration. Run as root on
# the target host.
#
# Safe to re-run.
#
# Usage:
#     sudo ./install.sh
#     curl -fsSL <hosted-url>/install.sh | sudo sh
#
# Environment variables:
#     CWAGENT_CLOUD         Target cloud: aws | azure | gcp (default: aws)
#     CWAGENT_AWS_ROLE_ARN  AWS IAM role ARN (required for azure and gcp)
#     CWAGENT_AWS_REGION    AWS region to send telemetry to (required for azure and gcp)

main() {
     set -eu

     DOWNLOAD_BASE="https://amazoncloudwatch-agent.s3.amazonaws.com"
     CLOUD="${CWAGENT_CLOUD:-aws}"
     ROLE_ARN="${CWAGENT_AWS_ROLE_ARN:-}"
     REGION="${CWAGENT_AWS_REGION:-}"
     INSTALL_ROOT="/opt/aws/amazon-cloudwatch-agent"
     CTL="${INSTALL_ROOT}/bin/amazon-cloudwatch-agent-ctl"

     # --- validate ---
     case "${CLOUD}" in
     aws | azure | gcp) ;;
     *) die "unsupported cloud '${CLOUD}' (expected: aws, azure, gcp)" ;;
     esac

     if [ "${CLOUD}" != "aws" ]; then
          [ -n "${ROLE_ARN}" ] || die "CWAGENT_AWS_ROLE_ARN is required for ${CLOUD} cloud"
          [ -n "${REGION}" ] || die "CWAGENT_AWS_REGION is required for ${CLOUD} cloud"
     fi

     [ "$(id -u)" -eq 0 ] || die "must be run as root"

     # --- detect arch ---
     case "$(uname -m)" in
     x86_64) ARCH="amd64" ;;
     aarch64) ARCH="arm64" ;;
     *) die "unsupported architecture $(uname -m)" ;;
     esac

     # --- detect package type ---
     PKGTYPE="$(detect_package_type)" || die "unable to detect a supported package manager"

     # --- download + install ---
     # To stderr: the download/install transcript (curl progress, rpm/dpkg
     # output) must stay off stdout, which carries only the success sentinel.
     install_"${PKGTYPE}" >&2

     # --- configure + start ---
     # Send the fetch-config transcript to stderr so stdout carries only the
     # status readout and success sentinel below.
     if [ "${CLOUD}" != "aws" ]; then
          # CWAGENT_ROLE_ARN here is the agent's own env var (its default:otel
          # config expands ${CWAGENT_ROLE_ARN}), not the CWAGENT_AWS_ROLE_ARN
          # input read above. Do not rename it to match.
          "${CTL}" -a set-env -e "CWAGENT_ROLE_ARN=${ROLE_ARN}" >&2
          "${CTL}" -a set-env -e "AWS_REGION=${REGION}" >&2
          "${CTL}" -a fetch-config -m auto -c default:otel -s >&2
     else
          "${CTL}" -a fetch-config -m ec2 -c default:otel -s >&2
     fi

     # fetch-config can exit 0 while leaving the agent stopped, so assert it is
     # actually running rather than trust the exit status. Guard the capture with
     # || true so a non-zero status still reaches the diagnostic below under set -e.
     STATUS="$("${CTL}" -a status)" || true
     if ! printf '%s' "${STATUS}" | grep -q '"status": *"running"'; then
          printf '%s\n' "${STATUS}" >&2
          die "agent did not start. Check ${INSTALL_ROOT}/logs/amazon-cloudwatch-agent.log"
     fi

     echo "Amazon CloudWatch Agent installed and running."
     printf '%s\n' "${STATUS}"
}

die() {
     echo "Error: $1" >&2
     exit 1
}

detect_package_type() {
     if [ -f /etc/os-release ]; then
          . /etc/os-release
          case "${ID:-}" in
          amzn | centos | rhel | fedora | rocky | almalinux | ol | sles | opensuse*)
               echo rpm
               return
               ;;
          ubuntu | debian | raspbian | pop | linuxmint)
               echo deb
               return
               ;;
          esac
     fi
     command -v rpm >/dev/null 2>&1 && {
          echo rpm
          return
     }
     command -v dpkg >/dev/null 2>&1 && {
          echo deb
          return
     }
     return 1
}

download() {
     url="$1"
     dest="$2"
     echo "Downloading ${url}"
     curl -fsSL "${url}" -o "${dest}" || die "failed to download ${url}"
}

install_rpm() {
     pkg="/tmp/amazon-cloudwatch-agent.rpm"
     download "${DOWNLOAD_BASE}/amazon_linux/${ARCH}/latest/amazon-cloudwatch-agent.rpm" "${pkg}"
     echo "Installing package..."
     rpm -Uvh --replacepkgs "${pkg}"
     rm -f "${pkg}"
}

install_deb() {
     pkg="/tmp/amazon-cloudwatch-agent.deb"
     download "${DOWNLOAD_BASE}/ubuntu/${ARCH}/latest/amazon-cloudwatch-agent.deb" "${pkg}"
     echo "Installing package..."
     dpkg -i "${pkg}"
     rm -f "${pkg}"
}

main
