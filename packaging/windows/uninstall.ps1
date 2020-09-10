# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$AmazonProgramFiles = "${Env:ProgramFiles}\Amazon"
$CWAProgramFiles = "${AmazonProgramFiles}\AmazonCloudWatchAgent"
$Cmd = "${CWAProgramFiles}\amazon-cloudwatch-agent-ctl.ps1"

if (Test-Path -LiteralPath "${Cmd}" -PathType Leaf) {
    & "${Cmd}" -Action prep-restart
    & "${Cmd}" -Action preun
}
if (Test-Path "${CWAProgramFiles}" -PathType Container) {
    Remove-Item -LiteralPath "${CWAProgramFiles}" -Force -Recurse
}

If (@(Get-ChildItem -LiteralPath "${AmazonProgramFiles}" -Force).Count -eq 0){
    Remove-Item -LiteralPath "${AmazonProgramFiles}" -Force -ErrorAction SilentlyContinue
}
