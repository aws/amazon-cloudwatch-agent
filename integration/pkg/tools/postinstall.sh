#!/bin/bash

echo "Creating a default user cwagent"
COMMON_CONFIG_PATH=/opt/aws/amazon-cloudwatch-agent/etc/common-config.toml
SAMPLE_SUFFIX=SAMPLE_DO_NOT_MODIFY
if [ ! -f ${COMMON_CONFIG_PATH} ]; then
     cp ${COMMON_CONFIG_PATH}.${SAMPLE_SUFFIX} ${COMMON_CONFIG_PATH}
fi

if [[ cwagent == $(sudo dscl . -list /Users UniqueID | awk '{print $1}' | grep -w cwagent) ]]; then
     echo "User already exists!"
     exit 0
fi

LastID=$(sudo dscl . -list /Users UniqueID | awk '{print $2}' | sort -n | tail -1)
NextID=$((LastID + 1))

. /etc/rc.common
sudo dscl . create /Users/cwagent
sudo dscl . create /Users/cwagent RealName cwagent

sudo dscl . create /Users/cwagent UniqueID $NextID
# PrimaryGroupID of 20 to create a standard user
sudo dscl . create /Users/cwagent PrimaryGroupID 20
sudo dscl . create /Users/cwagent UserShell /usr/bin/false
sudo dscl . create /Users/cwagent NFSHomeDirectory /Users/cwagent
sudo createhomedir -u cwagent -c

echo " "
echo "New user $(sudo dscl . -list /Users UniqueID | awk '{print $1}' | grep -w cwagent) has been created with unique ID $(sudo dscl . -list /Users UniqueID | grep -w cwagent | awk '{print $2}')"
