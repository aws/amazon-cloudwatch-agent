# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent — install (Windows)
#
# Downloads and installs the CloudWatch Agent MSI, then configures and
# starts it with the default OpenTelemetry (OTLP) configuration. Run as
# Administrator. Safe to re-run.
#
# Usage:
#   .\install.ps1
#   Invoke-WebRequest -Uri <hosted-url>/install.ps1 -OutFile $env:TEMP\install.ps1; & $env:TEMP\install.ps1
#
# Environment variables:
#   CWAGENT_INSTALL_URL   Override MSI download URL (pre-release testing)
#   CWAGENT_VERSION       Pin to a specific version (default: latest)
#   CWAGENT_CLOUD         Target cloud: aws | azure (default: aws)
#   CWAGENT_ROLE_ARN      AWS IAM role ARN (required for azure)
#   CWAGENT_AWS_REGION    AWS region to send telemetry to (required for azure)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$DownloadBase = "https://amazoncloudwatch-agent.s3.amazonaws.com"
$InstallUrl = if ($Env:CWAGENT_INSTALL_URL) { $Env:CWAGENT_INSTALL_URL } else { '' }
$Version = if ($Env:CWAGENT_VERSION) { $Env:CWAGENT_VERSION } else { 'latest' }
$Cloud = if ($Env:CWAGENT_CLOUD) { $Env:CWAGENT_CLOUD } else { 'aws' }
$RoleArn = if ($Env:CWAGENT_ROLE_ARN) { $Env:CWAGENT_ROLE_ARN } else { '' }
$Region = if ($Env:CWAGENT_AWS_REGION) { $Env:CWAGENT_AWS_REGION } else { '' }

$CWADirectory = 'Amazon\AmazonCloudWatchAgent'
$CWAProgramFiles = "${Env:ProgramFiles}\${CWADirectory}"
if ($Env:ProgramData) {
    $CWAProgramData = "${Env:ProgramData}\${CWADirectory}"
} else {
    # Windows 2003
    $CWAProgramData = "${Env:ALLUSERSPROFILE}\Application Data\${CWADirectory}"
}
$Ctl = "${CWAProgramFiles}\amazon-cloudwatch-agent-ctl.ps1"
$AgentExe = "${CWAProgramFiles}\amazon-cloudwatch-agent.exe"
$EnvConfig = "${CWAProgramData}\env-config.json"

# --- validate ---
if ($Cloud -notin @('aws', 'azure')) {
    Write-Error "unsupported cloud '${Cloud}' (expected: aws, azure)"
}
if ($Cloud -eq 'azure') {
    if (-not $RoleArn) {
        Write-Error "CWAGENT_ROLE_ARN is required for azure cloud"
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
$Url = if ($InstallUrl) {
    $InstallUrl
} else {
    "${DownloadBase}/windows/amd64/${Version}/amazon-cloudwatch-agent.msi"
}
Write-Output "Downloading ${Url}"
Invoke-WebRequest -Uri $Url -OutFile $MsiPath

# --- install ---
Write-Output "Installing package..."
$process = Start-Process msiexec.exe -ArgumentList "/i `"${MsiPath}`" /qn /norestart" -Wait -PassThru
if ($process.ExitCode -ne 0) {
    Write-Error "msiexec failed with exit code $($process.ExitCode)"
}
Remove-Item $MsiPath -Force -ErrorAction SilentlyContinue

# --- configure + start ---
if ($Cloud -eq 'azure') {
    & $AgentExe -setenv "CWAGENT_ROLE_ARN=${RoleArn}" -envconfig $EnvConfig
    & $AgentExe -setenv "AWS_REGION=${Region}" -envconfig $EnvConfig
    & $Ctl -Action fetch-config -Mode onPremise -ConfigLocation default:otel -Start
} else {
    & $Ctl -Action fetch-config -Mode ec2 -ConfigLocation default:otel -Start
}

Write-Output "Amazon CloudWatch Agent installed and running."
