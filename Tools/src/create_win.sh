#!/usr/bin/env bash
echo "****************************************"
echo "Creating zip file for Windows amd64"
echo "****************************************"
set -e

BUILD_ROOT="${BUILD_SPACE}/private/windows_${ARCH}/"
AGENT_VERSION=$(cat ${PREPKGPATH}/CWAGENT_VERSION)
echo "BUILD_SPACE: ${BUILD_SPACE}    agent_version: ${AGENT_VERSION}  pre-package location:${PREPKGPATH}"
echo "Creating windows folders"

mkdir -p "${BUILD_ROOT}/amazon-cloudwatch-agent"
mkdir -p ${BUILD_SPACE}/bin/windows/${ARCH}/
echo "Copying application files"

echo "Copying application files"
cp ${PREPKGPATH}/LICENSE ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/NOTICE ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/THIRD-PARTY-LICENSES ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/RELEASE_NOTES ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/CWAGENT_VERSION ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/amazon-cloudwatch-agent.exe ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-ctl.ps1 ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/install.ps1 ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/uninstall.ps1 ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/config-translator.exe ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/config-downloader.exe ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-config-wizard.exe ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/start-amazon-cloudwatch-agent.exe ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/common-config.toml ${BUILD_ROOT}/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-schema.json ${BUILD_ROOT}/amazon-cloudwatch-agent/

echo "Constructing the zip package"

if [ -f ${BUILD_ROOT}/amazon-cloudwatch-agent.zip ]; then
     rm ${BUILD_ROOT}/amazon-cloudwatch-agent.zip
fi
cd ${BUILD_ROOT}

zip -r amazon-cloudwatch-agent-${AGENT_VERSION}.zip *

mv ${BUILD_ROOT}/amazon-cloudwatch-agent-${AGENT_VERSION}.zip ${BUILD_SPACE}/bin/windows/${ARCH}/amazon-cloudwatch-agent.zip
ls -ltr ${BUILD_SPACE}/bin/windows/${ARCH}/amazon-cloudwatch-agent.zip
