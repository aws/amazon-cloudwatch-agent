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
