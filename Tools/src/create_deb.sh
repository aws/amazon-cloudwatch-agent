#!/usr/bin/env bash
echo "****************************************"
echo "Creating deb file for Debian Linux ${ARCH}"
echo "****************************************"
set -e

AGENT_VERSION=$(cat ${PREPKGPATH}/CWAGENT_VERSION)
BUILD_ROOT="${BUILD_SPACE}/private/linux_${ARCH}/debian"
echo "BUILD_SPACE: ${BUILD_SPACE}    agent_version: ${AGENT_VERSION}   pre-package location:${PREPKGPATH}"
echo "Creating debian folders"

mkdir -p ${BUILD_SPACE}/bin/linux/${ARCH}/
mkdir -p ${BUILD_ROOT}/bin
mkdir -p ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/logs
mkdir -p ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/var
mkdir -p ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.d
mkdir -p ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin
mkdir -p ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/doc
mkdir -p ${BUILD_ROOT}/etc/init
mkdir -p ${BUILD_ROOT}/etc/systemd/system/

echo "Copying application files"
cp ${PREPKGPATH}/LICENSE ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/NOTICE ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/THIRD-PARTY-LICENSES ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/RELEASE_NOTES ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/
cp ${PREPKGPATH}/CWAGENT_VERSION ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-ctl ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent.service ${BUILD_ROOT}/etc/systemd/system/
cp ${PREPKGPATH}/config-translator ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/config-downloader ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-config-wizard ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/start-amazon-cloudwatch-agent ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/
cp ${PREPKGPATH}/common-config.toml ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/etc/
cp ${PREPKGPATH}/amazon-cloudwatch-agent.conf ${BUILD_ROOT}/etc/init/
cp ${PREPKGPATH}/amazon-cloudwatch-agent-schema.json ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/doc/

############################# create the symbolic links here to make them managed by dpkg
# bin
mkdir -p ${BUILD_ROOT}/usr/bin
ln -f -s /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl ${BUILD_ROOT}/usr/bin/amazon-cloudwatch-agent-ctl
# etc
mkdir -p ${BUILD_ROOT}/etc/amazon
ln -f -s /opt/aws/amazon-cloudwatch-agent/etc ${BUILD_ROOT}/etc/amazon/amazon-cloudwatch-agent
# log
mkdir -p ${BUILD_ROOT}/var/log/amazon
ln -f -s /opt/aws/amazon-cloudwatch-agent/logs ${BUILD_ROOT}/var/log/amazon/amazon-cloudwatch-agent
# pid
mkdir -p ${BUILD_ROOT}/var/run/amazon
ln -f -s /opt/aws/amazon-cloudwatch-agent/var ${BUILD_ROOT}/var/run/amazon/amazon-cloudwatch-agent

cp ${BUILD_SPACE}/packaging/debian/conffiles ${BUILD_ROOT}/
cp ${BUILD_SPACE}/packaging/debian/preinst ${BUILD_ROOT}/
cp ${BUILD_SPACE}/packaging/debian/prerm ${BUILD_ROOT}/
cp ${BUILD_SPACE}/packaging/debian/debian-binary ${BUILD_ROOT}/

chmod ug+rx ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent
chmod ug+rx ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl
chmod ug+rx ${BUILD_ROOT}/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent

echo "Constructing the control file"
echo 'Package: amazon-cloudwatch-agent' >${BUILD_ROOT}/control
echo "Architecture: ${ARCH}" >>${BUILD_ROOT}/control
echo -n 'Version: ' >>${BUILD_ROOT}/control
echo -n ${AGENT_VERSION} >>${BUILD_ROOT}/control
echo '-1' >>${BUILD_ROOT}/control

cat ${BUILD_SPACE}/packaging/debian/control >>${BUILD_ROOT}/control

echo "Setting permissioning as required by debian"
cd ${BUILD_ROOT}/..
find ./debian -type d | xargs chmod 755
cd ~-

# the below permissioning is required by debian
cd ${BUILD_ROOT}
tar czf data.tar.gz opt etc usr var --owner=0 --group=0
cd ~-
cd ${BUILD_ROOT}
tar czf control.tar.gz control conffiles preinst prerm --owner=0 --group=0
cd ~-

echo "Creating the debian package"
echo "Constructing the deb packagage"
ar r ${BUILD_ROOT}/bin/amazon-cloudwatch-agent-${AGENT_VERSION}-1.deb ${BUILD_ROOT}/debian-binary
ar r ${BUILD_ROOT}/bin/amazon-cloudwatch-agent-${AGENT_VERSION}-1.deb ${BUILD_ROOT}/control.tar.gz
ar r ${BUILD_ROOT}/bin/amazon-cloudwatch-agent-${AGENT_VERSION}-1.deb ${BUILD_ROOT}/data.tar.gz

echo "Copying debian files to bin"

mv ${BUILD_ROOT}/bin/amazon-cloudwatch-agent-${AGENT_VERSION}-1.deb ${BUILD_SPACE}/bin/linux/${ARCH}/amazon-cloudwatch-agent.deb
ls -ltr ${BUILD_SPACE}/bin/linux/${ARCH}/*.deb
