# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

###########################################################
# This script is used for Sanity Checking for both CWAgent
# and ADOT Collector before running integration test
# https://github.com/aws/amazon-cloudwatch-agent/pull/478
##########################################################

$CWADirectory = 'Amazon\AmazonCloudWatchAgent'
$CWAProgramFiles = "${Env:ProgramFiles}\${CWADirectory}"

Function assertAgentsStatus(){
    Param (
        [Parameter(Mandatory = $true)]
        [string]$CWAgentRunningExpectedStatus,
        [Parameter(Mandatory = $true)]
        [string]$ADOTRunningExpectedStatus,
        [Parameter(Mandatory = $true)]
        [string]$CWAgentConfiguredExpectedStatus,
        [Parameter(Mandatory = $true)]
        [string]$ADOTConfiguredExpectedStatus
    )

    assertStatus -KeyToCheck "status" -ExpectedStatus "$CWAgentRunningExpectedStatus"
    assertStatus -KeyToCheck "cwoc_status" -ExpectedStatus "$ADOTRunningExpectedStatus"
    assertStatus -KeyToCheck "configstatus" -ExpectedStatus "$CWAgentConfiguredExpectedStatus"
    assertStatus -KeyToCheck "cwoc_configstatus" -ExpectedStatus "$ADOTConfiguredExpectedStatus"
}

Function assertStatus() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$KeyToCheck,
        [Parameter(Mandatory = $true)]
        [string]$ExpectedStatus
    )

    $KeysToCheck = @("status","configstatus","cwoc_status","cwoc_configstatus")
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

# Initial all setup for ADOT and CWAgent by removing all existing configuration
$step=0
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c all -o all
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop

$step=1
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a status
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" -ADOTRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "not configured" -ADOTConfiguredExpectedStatus "not configured"

$step=2
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a start
assertAgentsStatus -CWAgentRunningExpectedStatus "running" -ADOTRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "configured" -ADOTConfiguredExpectedStatus "not configured"

$step=3
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a fetch-config -o default -s
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c default -s
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" -ADOTRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "not configured" -ADOTConfiguredExpectedStatus "configured"

$step=4
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a fetch-config -c default -o invalid -s
assertAgentsStatus -CWAgentRunningExpectedStatus "running" -ADOTRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "configured" -ADOTConfiguredExpectedStatus "configured"

$step=5
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a prep-restart
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" -ADOTRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "configured" -ADOTConfiguredExpectedStatus "configured"
$step=6
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a cond-restart
assertAgentsStatus -CWAgentRunningExpectedStatus "running" -ADOTRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "configured" -ADOTConfiguredExpectedStatus "configured"

$step=7
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c default -s
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" -ADOTRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "not configured" -ADOTConfiguredExpectedStatus "configured"

$step=8
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -o default -s
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" -ADOTRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "not configured" -ADOTConfiguredExpectedStatus "not configured"

$step=9
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a append-config -c default -o default -s
assertAgentsStatus -CWAgentRunningExpectedStatus "running" -ADOTRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "configured" -ADOTConfiguredExpectedStatus "not configured"

$step=10
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c all
assertAgentsStatus -CWAgentRunningExpectedStatus "running" -ADOTRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "not configured" -ADOTConfiguredExpectedStatus "not configured"

$step=11
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a fetch-config -o default -s
assertAgentsStatus -CWAgentRunningExpectedStatus "running" -ADOTRunningExpectedStatus "running" `
                        -CWAgentConfiguredExpectedStatus "not configured" -ADOTConfiguredExpectedStatus "configured"

$step=12
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop
assertAgentsStatus -CWAgentRunningExpectedStatus "stopped" -ADOTRunningExpectedStatus "stopped" `
                        -CWAgentConfiguredExpectedStatus "not configured" -ADOTConfiguredExpectedStatus "configured"