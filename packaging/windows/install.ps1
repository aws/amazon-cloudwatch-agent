# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$CWADirectory = 'Amazon\AmazonCloudWatchAgent'

$CWAProgramFiles = "${Env:ProgramFiles}\${CWADirectory}"

if ($Env:ProgramData) {
    $CWAProgramData = "${Env:ProgramData}\${CWADirectory}"
} else {
    # Windows 2003
    $CWAProgramData = "${Env:ALLUSERSPROFILE}\Application Data\${CWADirectory}"
}

$Cmd = "${CWAProgramFiles}\amazon-cloudwatch-agent-ctl.ps1"

New-Item -ItemType Directory -Force -Path "${CWAProgramFiles}" | Out-Null
New-Item -ItemType Directory -Force -Path "${CWAProgramData}\Logs" | Out-Null
New-Item -ItemType Directory -Force -Path "${CWAProgramData}\Configs" | Out-Null

@(
"LICENSE",
"NOTICE",
"RELEASE_NOTES",
"CWAGENT_VERSION",
"amazon-cloudwatch-agent.exe",
"start-amazon-cloudwatch-agent.exe",
"amazon-cloudwatch-agent-ctl.ps1",
"config-downloader.exe",
"config-translator.exe",
"amazon-cloudwatch-agent-config-wizard.exe",
"amazon-cloudwatch-agent-schema.json"

) | ForEach-Object { Copy-Item ".\$_" -Destination "${CWAProgramFiles}" -Force }

@(
"common-config.toml"
) | ForEach-Object { Copy-Item ".\$_" -Destination "${CWAProgramData}" -Force }

& "${Cmd}" -Action cond-restart
