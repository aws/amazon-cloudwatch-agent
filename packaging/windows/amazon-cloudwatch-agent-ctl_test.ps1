# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Regression test for https://github.com/aws/amazon-cloudwatch-agent/issues/1218

$ErrorActionPreference = 'Stop'
$env:AMAZON_CLOUDWATCH_AGENT_CTL_SOURCE_ONLY = '1'
. (Join-Path $PSScriptRoot 'amazon-cloudwatch-agent-ctl.ps1')

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('n'))
New-Item -ItemType Directory -Path $tmp | Out-Null
$legacy = Join-Path $tmp 'amazon-cloudwatch-agent.json'

function Reset-Legacy {
    Set-Content -LiteralPath $legacy -Value '{}' -NoNewline
}

function Assert-Kept([string]$Location) {
    Reset-Legacy
    Remove-ReplacedLegacyJson -ConfigLocation $Location -LegacyJson $legacy
    if (-not (Test-Path -LiteralPath $legacy)) {
        throw "deleted source: $Location"
    }
}

function Assert-Removed([string]$Location) {
    Reset-Legacy
    Remove-ReplacedLegacyJson -ConfigLocation $Location -LegacyJson $legacy
    if (Test-Path -LiteralPath $legacy) {
        throw "kept legacy json for: $Location"
    }
}

try {
    Assert-Kept "file:$legacy"
    Assert-Kept ("file:" + $legacy.ToUpper())

    Push-Location $tmp
    try {
        Assert-Kept 'file:.\amazon-cloudwatch-agent.json'
    } finally {
        Pop-Location
    }

    $other = Join-Path $tmp 'other.json'
    Set-Content -LiteralPath $other -Value '{}' -NoNewline
    Assert-Removed "file:$other"
    Assert-Removed 'ssm:AmazonCloudWatch-Config.json'
    Assert-Removed 'default'
    Assert-Removed 'file:'

    Write-Output 'ok windows'
} finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
