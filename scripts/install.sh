#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent — install
#
# Downloads and installs the CloudWatch Agent package, then configures and
# starts it with the default OpenTelemetry (OTLP) configuration. Run as
# root on the target host. Safe to re-run.
#
# Usage:
#   sudo ./install.sh
#   curl -fsSL <hosted-url>/install.sh | sudo sh
#
# Environment variables:
#   CWAGENT_INSTALL_URL   Override package download URL (pre-release testing)
#   CWAGENT_VERSION       Pin to a specific version (default: latest)
#   CWAGENT_CLOUD         Target cloud: aws | azure (default: aws)
#   CWAGENT_ROLE_ARN      AWS IAM role ARN (required for azure)
#   CWAGENT_AWS_REGION    AWS region to send telemetry to (required for azure)

main() {
     set -eu

     DOWNLOAD_BASE="https://amazoncloudwatch-agent.s3.amazonaws.com"
     INSTALL_URL="${CWAGENT_INSTALL_URL:-}"
     VERSION="${CWAGENT_VERSION:-latest}"
     CLOUD="${CWAGENT_CLOUD:-aws}"
     ROLE_ARN="${CWAGENT_ROLE_ARN:-}"
     REGION="${CWAGENT_AWS_REGION:-}"
     INSTALL_ROOT="/opt/aws/amazon-cloudwatch-agent"
     CTL="${INSTALL_ROOT}/bin/amazon-cloudwatch-agent-ctl"
     ENV_CONFIG="${INSTALL_ROOT}/etc/env-config.json"

     # --- validate ---
     case "${CLOUD}" in
     aws | azure) ;;
     *) die "unsupported cloud '${CLOUD}' (expected: aws, azure)" ;;
     esac

     if [ "${CLOUD}" = "azure" ]; then
          [ -n "${ROLE_ARN}" ] || die "CWAGENT_ROLE_ARN is required for azure cloud"
          [ -n "${REGION}" ] || die "CWAGENT_AWS_REGION is required for azure cloud"
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
     install_"${PKGTYPE}"

     # --- configure + start ---
     if [ "${CLOUD}" = "azure" ]; then
          "${INSTALL_ROOT}/bin/amazon-cloudwatch-agent" -setenv "CWAGENT_ROLE_ARN=${ROLE_ARN}" -envconfig "${ENV_CONFIG}"
          "${INSTALL_ROOT}/bin/amazon-cloudwatch-agent" -setenv "AWS_REGION=${REGION}" -envconfig "${ENV_CONFIG}"
          "${CTL}" -a fetch-config -m onPremise -c default:otel -s
     else
          "${CTL}" -a fetch-config -m ec2 -c default:otel -s
     fi

     echo "Amazon CloudWatch Agent installed and running."
}

die() {
     echo "Error: $1" >&2
     exit 1
}

detect_package_type() {
     if [ -f /etc/os-release ]; then
          # shellcheck disable=SC1091
          . /etc/os-release
          case "$ID" in
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
     if [ -n "${INSTALL_URL}" ]; then
          download "${INSTALL_URL}" "${pkg}"
     else
          download "${DOWNLOAD_BASE}/amazon_linux/${ARCH}/${VERSION}/amazon-cloudwatch-agent.rpm" "${pkg}"
     fi
     echo "Installing package..."
     rpm -Uvh --replacepkgs "${pkg}"
     rm -f "${pkg}"
}

install_deb() {
     pkg="/tmp/amazon-cloudwatch-agent.deb"
     if [ -n "${INSTALL_URL}" ]; then
          download "${INSTALL_URL}" "${pkg}"
     else
          download "${DOWNLOAD_BASE}/ubuntu/${ARCH}/${VERSION}/amazon-cloudwatch-agent.deb" "${pkg}"
     fi
     echo "Installing package..."
     dpkg -i "${pkg}"
     rm -f "${pkg}"
}

main
