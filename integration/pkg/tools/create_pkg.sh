#!/usr/bin/env bash

#get the version of the agent
AGENT_VERSION=$(</tmp/CWAGENT_VERSION)

#create a .pkg file
rm -rf /tmp/AmazonCWAgentPackage
mkdir /tmp/AmazonCWAgentPackage
gunzip -c /tmp/amazon-cloudwatch-agent.tar.gz | tar -C /tmp/AmazonCWAgentPackage -xvf -
COMMON_CONFIG_PATH=/tmp/AmazonCWAgentPackage/opt/aws/amazon-cloudwatch-agent/etc/common-config.toml
SAMPLE_SUFFIX=SAMPLE_DO_NOT_MODIFY
mv ${COMMON_CONFIG_PATH} ${COMMON_CONFIG_PATH}.${SAMPLE_SUFFIX}
if [ $? -ne 0 ]; then
     echo "Failed to mv common-config.toml"
     exit 1
fi

mkdir /tmp/AmazonAgentScripts
mv preinstall.sh /tmp/AmazonAgentScripts/preinstall
mv postinstall.sh /tmp/AmazonAgentScripts/postinstall
chmod +x /tmp/AmazonAgentScripts/preinstall
chmod +x /tmp/AmazonAgentScripts/postinstall

rm -rf artifact
mkdir artifact
sudo pkgbuild --root /tmp/AmazonCWAgentPackage/ --install-location "/" --scripts /tmp/AmazonAgentScripts --identifier com.amazon.cloudwatch.agent --version=$AGENT_VERSION artifact/amazon-cloudwatch-agent.pkg
aws s3 cp ./artifact/amazon-cloudwatch-agent.pkg "s3://$1/integration-test/packaging/$2/amazon-cloudwatch-agent.pkg"

#TODO uncomment for mac specific signing gpg is supported
## create a package.tar.gz for the uploding it to signing bucket
#tar -cvzf artifact.gz -C artifact .
#tar -cvzf  package.tar.gz manifest.yaml artifact.gz
#
##upload the .pkg file created
#/usr/local/bin/aws s3 cp /tmp/package.tar.gz "s3://macos-cwagent-binaries/$AGENT_VERSION/pre-signed/package.tar.gz" --acl public-read
