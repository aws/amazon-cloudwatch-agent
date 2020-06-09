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

Param (
    [Parameter(Mandatory = $false)]
    [string]$Action,
    [Parameter(Mandatory = $false)]
    [switch]$Help,
    [Parameter(Mandatory = $false)]
    [string]$ConfigLocation = 'default',
    [Parameter(Mandatory = $false)]
    [switch]$Start = $false,
    [Parameter(Mandatory = $false)]
    [string]$Mode = 'ec2',
    [parameter(ValueFromRemainingArguments=$true)]
    $unsupportedVars
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$UsageString = @"


        usage: amazon-cloudwatch-agent-ctl.ps1 -a stop|start|status|fetch-config|append-config|remove-config [-m ec2|onPremise|auto] [-c default|ssm:<parameter-store-name>|file:<file-path>] [-s]

        e.g.
        1. apply a SSM parameter store config on EC2 instance and restart the agent afterwards:
            amazon-cloudwatch-agent-ctl.ps1 -a fetch-config -m ec2 -c ssm:AmazonCloudWatch-Config.json -s
        2. append a local json config file on onPremise host and restart the agent afterwards:
            amazon-cloudwatch-agent-ctl.ps1 -a append-config -m onPremise -c file:c:\config.json -s
        3. query agent status:
            amazon-cloudwatch-agent-ctl.ps1 -a status

        -a: action
            stop:                                   stop the agent process.
            start:                                  start the agent process.
            status:                                 get the status of the agent process.
            fetch-config:                           use this json config as the agent's only configuration.
            append-config:                          append json config with the existing json configs if any.
            remove-config:                          remove json config based on the location (ssm parameter store name, file name)

        -m: mode
            ec2:                                    indicate this is on ec2 host.
            onPremise:                              indicate this is on onPremise host.
            auto:                                   use ec2 metadata to determine the environment, may not be accurate if ec2 metadata is not available for some reason on EC2.

        -c: configuration
            default:                                default configuration for quick trial.
            ssm:<parameter-store-name>:             ssm parameter store name
            file:<file-path>:                       file path on the host

        -s: optionally restart after configuring the agent configuration
            this parameter is used for 'fetch-config', 'append-config', 'remove-config' action only.

"@

$CWAServiceName = 'AmazonCloudWatchAgent'
$CWAServiceDisplayName = 'Amazon CloudWatch Agent'
$CWADirectory = 'Amazon\AmazonCloudWatchAgent'

$CWAProgramFiles = "${Env:ProgramFiles}\${CWADirectory}"
if ($Env:ProgramData) {
    $CWAProgramData = "${Env:ProgramData}\${CWADirectory}"
} else {
    # Windows 2003
    $CWAProgramData = "${Env:ALLUSERSPROFILE}\Application Data\${CWADirectory}"
}

$CWALogDirectory = "${CWAProgramData}\Logs"

$RestartFile ="${CWAProgramData}\restart"
$VersionFile ="${CWAProgramFiles}\CWAGENT_VERSION"
$CVLogFile="${CWALogDirectory}\configuration-validation.log"

# The windows service registration assumes exactly this .toml file path and name
$TOML="${CWAProgramData}\amazon-cloudwatch-agent.toml"
$JSON="${CWAProgramData}\amazon-cloudwatch-agent.json"
$JSON_DIR = "${CWAProgramData}\Configs"
$COMMON_CONIG="${CWAProgramData}\common-config.toml"

$EC2 = $false
# WMI is unavailable on Nano, CIM is unavailable on 2003
$CIM = $false

Function CWAStart() {
    if (!(Test-Path -LiteralPath "${TOML}")) {
        Write-Output "amazon-cloudwatch-agent is not configured. Applying default configuration before starting it."
        CWAConfig
    }
    $svc = Get-Service -Name "${CWAServiceName}" -ErrorAction SilentlyContinue
    if (!$svc) {
        New-Service -Name "${CWAServiceName}" -DisplayName "${CWAServiceDisplayName}" -Description "${CWAServiceDisplayName}" -DependsOn LanmanServer -BinaryPathName "`"${CWAProgramFiles}\start-amazon-cloudwatch-agent.exe`"" | Out-Null
        # object returned by New-Service gives errors so retrieve it again
        $svc = Get-Service -Name "${CWAServiceName}"
        # Configure the service to restart on crashes. It's unclear how to do this through WMI or CIM interface so using sc.exe
        # Restarts immediately on the first two crashes then gives a 2 second sleep after any subsequent crash.
        & sc.exe failure "${CWAServiceName}" reset= 86400 actions= restart/0/restart/0/restart/2000 | Out-Null
        if ($CIM) {
            & sc.exe failureflag "${CWAServiceName}" 1 | Out-Null
        }
    }
    $svc | Start-Service
}

Function CWAStop() {
    $svc = Get-Service -Name "${CWAServiceName}" -ErrorAction SilentlyContinue
    if ($svc) {
        $svc | Stop-Service
    }
}

Function CWAPrepRestart() {
    if ((CWARunstatus) -eq 'running') {
        Write-Output $null > $RestartFile
    }
}

Function CWACondRestart() {
    if (Test-Path -LiteralPath "${RestartFile}") {
        CWAStart
        Remove-Item -LiteralPath "${RestartFile}"
    }
}

Function CWAPreun() {
    CWAStop
    if ($CIM) {
        $svc = Get-CimInstance -ClassName Win32_Service -Filter "name='${CWAServiceName}'"
        $svc | Invoke-CimMethod -MethodName 'delete' | Out-Null
    } else {
        $svc = Get-WmiObject -Class Win32_Service -Filter "name='${CWAServiceName}'"
        $svc.delete() | Out-Null
    }
}

Function CWAStatus() {

    $timefmt=''

    if ($CIM) {
        $svc = Get-CimInstance -ClassName Win32_Service -Filter "name='${CWAServiceName}'"
    } else {
        $svc = Get-WmiObject -Class Win32_Service -Filter "name='${CWAServiceName}'"
    }

    if ($svc) {
        $cwapid = $svc.ProcessId
        $process = Get-Process -Id "${cwapid}"
        $processStart = $process.StartTime
        if ($processStart) {
            $timefmt = Get-Date -Date ${processStart} -Format "s"
        }
    }
    
    $status = CWARunstatus
    $version = ([IO.File]::ReadAllText("${VersionFile}")).Trim()
    
    Write-Output "{"
    Write-Output "  `"status`": `"${status}`","
    Write-Output "  `"starttime`": `"${timefmt}`","
    Write-Output "  `"version`": `"${version}`""
    Write-Output "}"
}

# Translate platform status names to those used across all CWAgent's platforms
Function CWARunstatus() {
    $running = $false
    $svc = Get-Service -Name "${CWAServiceName}" -ErrorAction SilentlyContinue
    if ($svc -and ($svc.Status -eq 'running')) {
        $running = $true
    }
    if ($running) {
        return 'running'
    } else {
        return 'stopped'
    }
}

Function CWAConfig() {
    Param (
        [Parameter(Mandatory = $false)]
        [string]$multi_config = 'default'
    )

    $param_mode="ec2"
    if (!$EC2) {
        $param_mode="onPrem"
    }

    & $CWAProgramFiles\config-downloader.exe --output-dir "${JSON_DIR}" --download-source "${ConfigLocation}" --mode "${param_mode}" --config "${COMMON_CONIG}" --multi-config "${multi_config}"
    CheckCMDResult
    Write-Output "Start configuration validation..."
    & cmd /c "`"$CWAProgramFiles\config-translator.exe`" --input ${JSON} --input-dir ${JSON_DIR} --output ${TOML} --mode ${param_mode} --config ${COMMON_CONIG} --multi-config ${multi_config} 2>&1"
    CheckCMDResult
    # Let command pass so we can check return code and give user-friendly error-message
    $ErrorActionPreference = "Continue"
    & cmd /c "`"${CWAProgramFiles}\amazon-cloudwatch-agent.exe`" --schematest --config ${TOML} 2>&1" | Out-File $CVLogFile
    if ($LASTEXITCODE -ne 0) {
        Write-Output "Configuration validation second phase failed"
        Write-Output "======== Error Log ========"
        cat $CVLogFile
        exit 1
    } else {
        Write-Output "Configuration validation second phase succeeded"
    }
    $ErrorActionPreference = "Stop"
    Write-Output "Configuration validation succeeded"

    # for translator:
    #       default:    only process .tmp files
    #       append:     process both existing files and .tmp files
    #       remove:     only process existing files
    # At this point, all json configs have been validated
    # multi_config:
    #       default:    delete non .tmp file, rename .tmp file
    #       append:     rename .tmp file
    #       remove:     no-op
    if ($multi_config -eq 'default') {
        Remove-Item "${JSON}" -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "${JSON_DIR}\*" -Exclude "*.tmp" -Force -ErrorAction SilentlyContinue
        Get-ChildItem "${JSON_DIR}\*.tmp" | Rename-Item -NewName { $_.name -Replace '\.tmp$','' }
    } elseif ($multi_config -eq 'append') {
        Get-ChildItem "${JSON_DIR}\*.tmp" | ForEach-Object {
            $newName = $_.name -Replace  '\.tmp$',''
            $destination = Join-Path -Path $_.Directory.FullName -ChildPath "${newName}"
            Move-Item -Path $_.FullName -Destination "${destination}" -Force
        }
    }

    if ($Start) {
        CWAStop
        CWAStart
    }
}

# For exes(non cmlet) the $ErrorActionPreference won't help if run cmd result failed,
# We have to check the $LASTEXITCODE everytime.
Function CheckCMDResult($ErrorMessag, $SucessMessage) {
    if ($LASTEXITCODE -ne 0) {
        if (![string]::IsNullOrEmpty($ErrorMessag)) {
            Write-Output $ErrorMessag
        }
        exit 1
    } else {
        if (![string]::IsNullOrEmpty($SucessMessage)) {
            Write-Output $SucessMessage
        }
    }

}

# TODO Occasionally metadata service isn't available and this gives a false negative - might
# be a better way to probe
# http://docs.aws.amazon.com/AWSEC2/latest/WindowsGuide/identify_ec2_instances.html
# Ultimately though an optional 'ec2-override' flag seems necessary for easier testing
Function CWATestEC2() {
    $error.clear()
    $request = [System.Net.WebRequest]::Create('http://169.254.169.254/')
    $request.Timeout = 5
    try {
        $response = $request.GetResponse()
        $response.Close()
    } catch {
        return $false
    }
    return !$error
}

Function main() {

    if (Get-Command 'Get-CimInstance' -CommandType Cmdlet -ErrorAction SilentlyContinue) {
        $CIM = $true
    }

    if ($unsupportedVars) {
        Write-Output "Ignore unsupported params: $unsupportedVars`n${UsageString}"
    }

    if ($Help) {
        Write-Output "${UsageString}"
        exit 0
    }

    switch -exact ($Mode) {
        ec2 { $EC2 = $true }
        onPremise { $EC2 = $false }
        auto { $EC2 = CWATestEC2 }
        default {
           Write-Output "Invalid mode: ${Mode}`n${UsageString}"
           Exit 1
        }
    }

    switch -exact ($Action) {
        stop { CWAStop }
        start { CWAStart }
        fetch-config { CWAConfig }
        append-config { CWAConfig -multi_config 'append' }
        remove-config { CWAConfig -multi_config 'remove' }
        status { CWAStatus }
        prep-restart { CWAPrepRestart }
        cond-restart { CWACondRestart }
        preun { CWAPreun }
        default {
           Write-Output "Invalid action: ${Action}`n${UsageString}"
           Exit 1
        }
    }
}

main
