#!/bin/bash


echo "Moving amazon-cloudwatch-agent-ctl binary and config.json to /tmp..."
sudo mv /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl /tmp/
sudo mv /opt/aws/amazon-cloudwatch-agent/bin/config.json /tmp/
sudo mv /opt/aws/amazon-cloudwatch-agent/bin/CWAGENT_VERSION /tmp/
# Step 3: Remove everything from /opt/aws/amazon-cloudwatch-agent/bin
echo "Removing everything from /opt/aws/amazon-cloudwatch-agent/bin..."
sudo rm -rf /opt/aws/amazon-cloudwatch-agent/bin/*

# Step 4: Replace everything in bin with contents from /local/home/siprmp/amazon-cloudwatch-agent/build/bin/linux_amd64/
echo "Copying files from /local/home/siprmp/amazon-cloudwatch-agent/build/bin/linux_amd64/ to /opt/aws/amazon-cloudwatch-agent/bin/..."
sudo cp -r /home/ec2-user/amazon-cloudwatch-agent/build/bin/linux_amd64/* /opt/aws/amazon-cloudwatch-agent/bin/

# Step 5: Move amazon-cloudwatch-agent-ctl binary and config.json back to the bin directory
echo "Moving amazon-cloudwatch-agent-ctl binary and config.json back to /opt/aws/amazon-cloudwatch-agent/bin/..."
sudo mv /tmp/amazon-cloudwatch-agent-ctl /opt/aws/amazon-cloudwatch-agent/bin/
sudo mv /tmp/config.json /opt/aws/amazon-cloudwatch-agent/bin/
sudo mv /tmp/CWAGENT_VERSION /opt/aws/amazon-cloudwatch-agent/bin/

# Step 6: Stop CloudWatch Agent
echo "Stopping CloudWatch Agent..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a stop

# Step 7: Remove CloudWatch Agent logs
echo "Removing CloudWatch Agent logs..."
sleep 2
sudo rm /opt/aws/amazon-cloudwatch/logs/amazon-cloudwatch-agent.log
sleep 5
# Step 8: Print the status of CloudWatch Agent
echo "Printing CloudWatch Agent status..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a status

# Step 9: Echo starting the agent
echo "Starting the CloudWatch Agent..."

echo "Printing CloudWatch Agent status before start..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a status

sudo chmod 777 -R /opt/aws/amazon-cloudwatch-agent/bin

sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -s -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/bin/config.json

echo "Printing CloudWatch Agent status after start..."
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -m ec2 -a status

echo "Printing CloudWatch Agent logs..."
sudo tail -f /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log