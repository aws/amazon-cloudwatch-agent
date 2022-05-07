#!/bin/bash

COMMON_CONFIG_PATH=/opt/aws/amazon-cloudwatch-agent/etc/common-config.toml
SAMPLE_SUFFIX=SAMPLE_DO_NOT_MODIFY
if [ -e ${COMMON_CONFIG_PATH}.${SAMPLE_SUFFIX} -a -e ${COMMON_CONFIG_PATH} ]; then
     diff ${COMMON_CONFIG_PATH}.${SAMPLE_SUFFIX} ${COMMON_CONFIG_PATH} >/dev/null 2>&1
     if [ $? -eq 0 ]; then
          rm -r ${COMMON_CONFIG_PATH}
     fi
fi

launchctl list com.amazon.cloudwatch.agent >/dev/null 2>&1
if [ $? -eq 0 ]; then
     echo "Agent is running in the instance"
     echo "Stopping the agent"
     launchctl unload /Library/LaunchDaemons/com.amazon.cloudwatch.agent.plist
     echo "Agent stopped"
fi
