#!/usr/bin/env bash
echo "*************************************************"
echo "Creating rpm file for Amazon Linux and RHEL ${ARCH}"
echo "*************************************************"
set -e

SPEC_FILE="${PREPKGPATH}/amazon-cloudwatch-agent.spec"
BUILD_ROOT="${BUILD_SPACE}/private/linux_${ARCH}/rpm-build"
AGENT_VERSION=$(cat ${PREPKGPATH}/CWAGENT_VERSION | sed -e "s/-/+/g")

echo "BUILD_SPACE: ${BUILD_SPACE}  agent_version: ${AGENT_VERSION}  pre-package location:${PREPKGPATH}"

echo "Creating rpm-build workspace"

mkdir -p ${BUILD_SPACE}/bin/linux/${ARCH}/
mkdir -p ${BUILD_ROOT}/{RPMS,SRPMS,BUILD,SOURCES,SPECS,BUILDROOT}
mkdir -p ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/logs
mkdir -p ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/var
mkdir -p ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.d
mkdir -p ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin
mkdir -p ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/doc
mkdir -p ${BUILD_ROOT}/SOURCES/etc/init
mkdir -p ${BUILD_ROOT}/SOURCES/etc/systemd/system/

echo "Copying application files"

cp ${PREPKGPATH}/LICENSE ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/NOTICE ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/THIRD-PARTY-LICENSES ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/RELEASE_NOTES ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/CWAGENT_VERSION ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-ctl ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent.service ${BUILD_ROOT}/SOURCES/etc/systemd/system/
cp ${PREPKGPATH}/config-translator ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/config-downloader ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-config-wizard ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/start-amazon-cloudwatch-agent ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/common-config.toml ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/etc/
cp ${PREPKGPATH}/amazon-cloudwatch-agent.conf ${BUILD_ROOT}/SOURCES/etc/init/amazon-cloudwatch-agent.conf
cp ${PREPKGPATH}/amazon-cloudwatch-agent-schema.json ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/doc/

chmod ug+rx ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent
chmod ug+rx ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl
chmod ug+rx ${BUILD_ROOT}/SOURCES/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent
tar -zcvf ${BUILD_ROOT}/SOURCES/amazon-cloudwatch-agent.tar.gz -C ${BUILD_ROOT}/SOURCES opt etc

rm -rf ${BUILD_ROOT}/SOURCES/opt ${BUILD_ROOT}/SOURCES/etc

echo "Creating the rpm package"

rpmbuild -bb -v --clean --define "AGENT_VERSION $AGENT_VERSION" --define "_topdir ${BUILD_ROOT}" ${SPEC_FILE} --target ${TARGET_SUPPORTED_ARCH}

echo "Copying rpm files to bin"

mv ${BUILD_ROOT}/RPMS/${TARGET_SUPPORTED_ARCH}/amazon-cloudwatch-agent-${AGENT_VERSION}-1.${TARGET_SUPPORTED_ARCH}.rpm ${BUILD_SPACE}/bin/linux/${ARCH}/amazon-cloudwatch-agent.rpm
ls -ltr ${BUILD_SPACE}/bin/linux/${ARCH}/*.rpm
