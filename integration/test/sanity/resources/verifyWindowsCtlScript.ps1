# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

###########################################################
# This script is used for Sanity Checking the CWAgent
# https://github.com/aws/amazon-cloudwatch-agent/pull/478
##########################################################

$CWADirectory = 'Amazon\AmazonCloudWatchAgent'
$CWAProgramFiles = "${Env:ProgramFiles}\${CWADirectory}"

Function assertAgentsStatus(){
    Param (
        [Parameter(Mandatory = $true)]
        [string]$CWAgentRunningExpectedStatus,
        [Parameter(Mandatory = $true)]
        [string]$CWAgentConfiguredExpectedStatus
    )

    assertStatus -KeyToCheck "status" -ExpectedStatus "$CWAgentRunningExpectedStatus"
    assertStatus -KeyToCheck "configstatus" -ExpectedStatus "$CWAgentConfiguredExpectedStatus"
}

Function assertStatus() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$KeyToCheck,
        [Parameter(Mandatory = $true)]
        [string]$ExpectedStatus
    )

    $KeysToCheck = @("status","configstatus")
    if (-Not ($KeysToCheck -contains $KeyToCheck)){
        Write-Output "Invalid KeyToCheck: $KeyToCheck, only supports $KeysToCheck"
        Exit 1
    }

	$OutputStatus = (& "${CWAProgramFiles}\amazon-cloudwatch-agent-ctl.ps1" -a status | ConvertFrom-Json)."$KeyToCheck"
	if  ( -Not $outputStatus.equals($ExpectedStatus) ) {
	    Write-Output "In step ${step}, ${KeyToCheck} is NOT expected. (actual=`"${OutputStatus}`"; expected=`"${ExpectedStatus}`")"
    	Exit 1
	}

	Write-Output "In step ${step}, ${KeyToCheck} is expected"
}

# Initial all setup for CWAgent by removing all existing configuration
$step=0
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c all
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop

$step=1
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a status
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped"  `
                        -CWAgentConfiguredExpectedStatus "not configured"

$step=2
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a start
assertAgentsStatus -CWAgentRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "configured"

$step=3
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c default -s
assertAgentsStatus -CWAgentRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "configured"

$step=4
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a prep-restart
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "configured"

$step=5
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a cond-restart
assertAgentsStatus -CWAgentRunningExpectedStatus "running"  `
                        -CWAgentConfiguredExpectedStatus "configured"

$step=6
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a append-config -c default -s
assertAgentsStatus -CWAgentRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "configured"

$step=7
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c all
assertAgentsStatus -CWAgentRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "not configured"

$step=8
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a fetch-config -s
assertAgentsStatus -CWAgentRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "configured"

$step=9
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c all
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "not configured"