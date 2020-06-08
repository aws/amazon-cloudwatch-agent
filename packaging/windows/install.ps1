# Copyright 2017 Amazon.com, Inc. and its affiliates. All Rights Reserved.
#
# Licensed under the Amazon Software License (the "License").
# You may not use this file except in compliance with the License.
# A copy of the License is located at
#
#   http://aws.amazon.com/asl/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

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
