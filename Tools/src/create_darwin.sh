#!/bin/bash

#File Paths
CONFIG_JSON="/opt/aws/amazon-cloudwatch-agent/bin/config.json"
CONFIG_TOML="/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.toml"
CONFIG_YAML="/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.yaml"
LOG_FILE="/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"
STATUS_CMD="amazon-cloudwatch-agent-ctl -a status"
OUTPUT_FILE="debug-output.txt"
TARBALL="debug-files.tar.gz"

echo "==== CloudWatch Agent Debug Information ====" > $OUTPUT_FILE
echo "" >> $OUTPUT_FILE

tar -cf $TARBALL --files-from /dev/null

#Collect configuration files and include them in the tarball
for CONFIG_FILE in "$CONFIG_JSON" "$CONFIG_TOML" "$CONFIG_YAML"; do
    if [ -f "$CONFIG_FILE" ]; then
        echo "==== Contents of $CONFIG_FILE ====" >> $OUTPUT_FILE
        cat "$CONFIG_FILE" >> $OUTPUT_FILE
        echo "" >> $OUTPUT_FILE
        # Use -C to change to the base directory and add files relative to it
        tar -C /opt/aws/amazon-cloudwatch-agent -rf $TARBALL "${CONFIG_FILE#/opt/aws/amazon-cloudwatch-agent/}"
    else
        echo "==== $CONFIG_FILE not found ====" >> $OUTPUT_FILE
        echo "" >> $OUTPUT_FILE
    fi
done

#Collect agent status and add it to the output file
echo "==== CloudWatch Agent Status ====" >> $OUTPUT_FILE
if $STATUS_CMD &>/dev/null; then
    $STATUS_CMD >> $OUTPUT_FILE
else
    echo "Status command failed or is unavailable." >> $OUTPUT_FILE
fi
echo "" >> $OUTPUT_FILE

#Handle the log file
if [ -f "$LOG_FILE" ]; then
    echo "Log file found: $LOG_FILE" >> $OUTPUT_FILE
    echo "Please share the log file separately or use the generated tarball." >> $OUTPUT_FILE
    # Use -C to change to the base directory and add the log file relative to it
    tar -C /opt/aws/amazon-cloudwatch-agent -rf $TARBALL "${LOG_FILE#/opt/aws/amazon-cloudwatch-agent/}"
else
    echo "Log file not found: $LOG_FILE" >> $OUTPUT_FILE
fi

#Compress the tarball
gzip -f $TARBALL

#Notify the user
echo "Debugging information collected:"
echo "- All configurations and logs (if available) are in $TARBALL.gz"
echo "- Summary information is in $OUTPUT_FILE"
echo ""
echo "Please share the contents of $OUTPUT_FILE by copying and pasting it."
echo "Alternatively, upload $TARBALL.gz to a file-sharing platform and share the link."

