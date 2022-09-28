#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail
echo "****************************************"
echo "Creating tar file for Mac OS X ${ARCH}  "
echo "****************************************"

AGENT_VERSION=$(cat ${PREPKGPATH}/CWAGENT_VERSION | sed -e "s/-/+/g")
echo "BUILD_SPACE: ${BUILD_SPACE}    agent_version: ${AGENT_VERSION}  pre-package location:${PREPKGPATH}"

mkdir -p ${BUILD_SPACE}/bin/darwin/${ARCH}/

echo "Creating darwin folders"
MACHINE_ROOT="/opt/aws/amazon-cloudwatch-agent/"
BUILD_ROOT="${BUILD_SPACE}/private/darwin_${ARCH}"
TAR_NAME="amazon-cloudwatch-agent.tar.gz"

echo "Creating darwin workspace"
mkdir -p ${BUILD_ROOT}${MACHINE_ROOT}logs
mkdir -p ${BUILD_ROOT}${MACHINE_ROOT}bin
mkdir -p ${BUILD_ROOT}${MACHINE_ROOT}etc
mkdir -p ${BUILD_ROOT}${MACHINE_ROOT}etc/amazon-cloudwatch-agent.d
mkdir -p ${BUILD_ROOT}${MACHINE_ROOT}var
mkdir -p ${BUILD_ROOT}${MACHINE_ROOT}doc
mkdir -p ${BUILD_ROOT}/Library/LaunchDaemons

############################# create the symbolic links
# log
mkdir -p ${BUILD_ROOT}/var/log/amazon
ln -f -s /opt/aws/amazon-cloudwatch-agent/logs ${BUILD_ROOT}/var/log/amazon/amazon-cloudwatch-agent

echo "Copying application files"
cp ${PREPKGPATH}/LICENSE ${BUILD_ROOT}${MACHINE_ROOT}
cp ${PREPKGPATH}/NOTICE ${BUILD_ROOT}${MACHINE_ROOT}
cp ${PREPKGPATH}/THIRD-PARTY-LICENSES ${BUILD_ROOT}${MACHINE_ROOT}
cp ${PREPKGPATH}/RELEASE_NOTES ${BUILD_ROOT}${MACHINE_ROOT}
cp ${PREPKGPATH}/CWAGENT_VERSION ${BUILD_ROOT}${MACHINE_ROOT}bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent ${BUILD_ROOT}${MACHINE_ROOT}bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-ctl ${BUILD_ROOT}${MACHINE_ROOT}bin/
cp ${PREPKGPATH}/config-translator ${BUILD_ROOT}${MACHINE_ROOT}bin/
cp ${PREPKGPATH}/config-downloader ${BUILD_ROOT}${MACHINE_ROOT}bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-config-wizard ${BUILD_ROOT}${MACHINE_ROOT}bin/
cp ${PREPKGPATH}/start-amazon-cloudwatch-agent ${BUILD_ROOT}${MACHINE_ROOT}bin/
cp ${PREPKGPATH}/common-config.toml ${BUILD_ROOT}${MACHINE_ROOT}etc/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-schema.json ${BUILD_ROOT}${MACHINE_ROOT}doc/
cp ${PREPKGPATH}/com.amazon.cloudwatch.agent.plist ${BUILD_ROOT}/Library/LaunchDaemons/

echo "Setting permissions as required by launchd"
chmod 600 ${BUILD_ROOT}/Library/LaunchDaemons/*
chmod ug+rx ${BUILD_ROOT}${MACHINE_ROOT}bin/amazon-cloudwatch-agent
chmod ug+rx ${BUILD_ROOT}${MACHINE_ROOT}bin/amazon-cloudwatch-agent-ctl
chmod ug+rx ${BUILD_ROOT}${MACHINE_ROOT}bin/start-amazon-cloudwatch-agent

echo "Creating tar"
(
     cd ${BUILD_ROOT}
     tar -czf $TAR_NAME *
)

echo "Archive created at ${BUILD_ROOT}/${TAR_NAME}"

echo "Copying tarball to bin"
mv ${BUILD_ROOT}/${TAR_NAME} ${BUILD_SPACE}/bin/darwin/${ARCH}/${TAR_NAME}
ls -ltr ${BUILD_SPACE}/bin/darwin/${ARCH}/*.tar.gz
