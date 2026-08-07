# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent - install (Windows)
#
# Downloads and installs the CloudWatch Agent MSI, then configures and
# starts it with the default OpenTelemetry (OTLP) configuration. Run as
# Administrator.
#
# Safe to re-run.
#
# Usage:
#     .\install.ps1
#     Invoke-WebRequest -Uri <hosted-url>/install.ps1 -OutFile $env:TEMP\install.ps1; & $env:TEMP\install.ps1
#
# Environment variables:
#     CWAGENT_CLOUD         Target cloud: aws | azure (default: aws)
#     CWAGENT_AWS_ROLE_ARN  AWS IAM role ARN (required for azure)
#     CWAGENT_AWS_REGION    AWS region to send telemetry to (required for azure)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$DownloadBase = "https://amazoncloudwatch-agent.s3.amazonaws.com"
$Cloud = if ($Env:CWAGENT_CLOUD) { $Env:CWAGENT_CLOUD } else { 'aws' }
$RoleArn = if ($Env:CWAGENT_AWS_ROLE_ARN) { $Env:CWAGENT_AWS_ROLE_ARN } else { '' }
$Region = if ($Env:CWAGENT_AWS_REGION) { $Env:CWAGENT_AWS_REGION } else { '' }

$CWADirectory = 'Amazon\AmazonCloudWatchAgent'
$CWAProgramFiles = "${Env:ProgramFiles}\${CWADirectory}"
$Ctl = "${CWAProgramFiles}\amazon-cloudwatch-agent-ctl.ps1"

# --- validate ---
if ($Cloud -notin @('aws', 'azure')) {
    Write-Error "unsupported cloud '${Cloud}' (expected: aws, azure)"
}
if ($Cloud -eq 'azure') {
    if (-not $RoleArn) {
        Write-Error "CWAGENT_AWS_ROLE_ARN is required for azure cloud"
    }
    if (-not $Region) {
        Write-Error "CWAGENT_AWS_REGION is required for azure cloud"
    }
}

$identity = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $identity.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "must be run as Administrator"
}

# --- download ---
$MsiPath = "${Env:TEMP}\amazon-cloudwatch-agent.msi"
$Url = "${DownloadBase}/windows/amd64/latest/amazon-cloudwatch-agent.msi"
[Console]::Error.WriteLine("Downloading ${Url}")
# Windows PowerShell 5.1 (the default under az vm run-command / EC2) negotiates
# TLS 1.0 by default, which S3 rejects, so force TLS 1.2. -UseBasicParsing avoids the
# IE engine, absent on Server Core.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri $Url -OutFile $MsiPath -UseBasicParsing

# --- install ---
[Console]::Error.WriteLine("Installing package...")
$process = Start-Process msiexec.exe -ArgumentList "/i `"${MsiPath}`" /qn /norestart" -Wait -PassThru
# 3010/1641 are success-with-reboot-required codes (/norestart returns them), not failures.
if ($process.ExitCode -notin @(0, 3010, 1641)) {
    Write-Error "msiexec failed with exit code $($process.ExitCode)"
}
Remove-Item $MsiPath -Force -ErrorAction SilentlyContinue

# --- configure + start ---
# Send the fetch-config transcript to stderr so stdout carries only the status
# readout and success sentinel below.
if ($Cloud -eq 'azure') {
    # CWAGENT_ROLE_ARN here is the agent's own env var (its default:otel config
    # expands ${CWAGENT_ROLE_ARN}), not the CWAGENT_AWS_ROLE_ARN input read
    # above. Do not rename it to match.
    & $Ctl -Action set-env -EnvVar "CWAGENT_ROLE_ARN=${RoleArn}" 2>&1 | ForEach-Object { [Console]::Error.WriteLine($_) }
    & $Ctl -Action set-env -EnvVar "AWS_REGION=${Region}" 2>&1 | ForEach-Object { [Console]::Error.WriteLine($_) }
    & $Ctl -Action fetch-config -Mode auto -ConfigLocation default:otel -Start 2>&1 | ForEach-Object { [Console]::Error.WriteLine($_) }
} else {
    & $Ctl -Action fetch-config -Mode ec2 -ConfigLocation default:otel -Start 2>&1 | ForEach-Object { [Console]::Error.WriteLine($_) }
}

# fetch-config can exit 0 while leaving the agent stopped, so assert it is
# actually running rather than trust the exit status.
$Status = (& $Ctl -Action status | Out-String)
if (-not ($Status -match '"status":\s*"running"')) {
    [Console]::Error.WriteLine($Status)
    Write-Error "agent did not start. Check the amazon-cloudwatch-agent log"
}

Write-Output "Amazon CloudWatch Agent installed and running."
Write-Output $Status.TrimEnd()
