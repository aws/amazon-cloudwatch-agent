# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

Param (
    [Parameter(Mandatory = $false)]
    [string]$Action,
    [Parameter(Mandatory = $false)]
    [switch]$Help,
    [Parameter(Mandatory = $false)]
    [string]$ConfigLocation = '',
    [Parameter(Mandatory = $false)]
    [string]$OtelConfigLocation = '',
    [Parameter(Mandatory = $false)]
    [switch]$Start = $false,
    [Parameter(Mandatory = $false)]
    [string]$Mode = 'ec2',
    [Parameter(Mandatory = $false)]
    [string]$LogLevel = '',
    [parameter(ValueFromRemainingArguments=$true)]
    $unsupportedVars
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$UsageString = @"


        usage:  amazon-cloudwatch-agent-ctl.ps1 -a
                stop|start|status|fetch-config|append-config|remove-config|set-log-level
                [-m ec2|onPremise|auto]
                [-c default|all|ssm:<parameter-store-name>|file:<file-path>]
                [-o default|all|ssm:<parameter-store-name>|file:<file-path>]
                [-s]
                [-l INFO|DEBUG|WARN|ERROR|OFF]

        e.g.
        1. apply a SSM parameter store config on EC2 instance and restart the agent afterwards:
            amazon-cloudwatch-agent-ctl.ps1 -a fetch-config -m ec2 -c ssm:AmazonCloudWatch-Config.json -s
        2. append a local json config file on onPremise host and restart the agent afterwards:
            amazon-cloudwatch-agent-ctl.ps1 -a append-config -m onPremise -c file:c:\config.json -s
        3. query agent status:
            amazon-cloudwatch-agent-ctl.ps1 -a status

        -a: action
            stop:                                   stop both amazon-cloudwatch-agent and cwagent-otel-collector if running.
            start:                                  start both amazon-cloudwatch-agent and cwagent-otel-collector if configuration is available.
            status:                                 get the status of both agent processes.
            fetch-config:                           apply config for agent, followed by -c or -o or both. Target config can be based on location (ssm parameter store name, file name), or 'default'.
            append-config:                          append json config with the existing json configs if any, followed by -c. Target config can be based on the location (ssm parameter store name, file name), or 'default'.
            remove-config:                          remove config for agent, followed by -c or -o or both. Target config can be based on the location (ssm parameter store name, file name), or 'all'.
            set-log-level:                          sets the log level, followed by -l to provide the level in all caps.

        -m: mode
            ec2:                                    indicate this is on ec2 host.
            onPremise:                              indicate this is on onPremise host.
            auto:                                   use ec2 metadata to determine the environment, may not be accurate if ec2 metadata is not available for some reason on EC2.

        -c: amazon-cloudwatch-agent configuration
            default:                                default configuration for quick trial.
            ssm:<parameter-store-name>:             ssm parameter store name.
            file:<file-path>:                       file path on the host.
            all:                                    all existing configs. Only apply to remove-config action.

        -o: cwagent-otel-collector configuration
            default:                                default configuration for quick trial.
            ssm:<parameter-store-name>:             ssm parameter store name.
            file:<file-path>:                       file path on the host.
            all:                                    all existing configs. Only apply to remove-config action.

        -s: optionally restart after configuring the agent configuration
            this parameter is used for 'fetch-config', 'append-config', 'remove-config' action only.

        -l: log level to set the agent to INFO, DEBUG, WARN, ERROR, or OFF
            this parameter is used for 'set-log-level' only.

"@

$CWAServiceName = 'AmazonCloudWatchAgent'
$CWOCServiceName = 'CWAgentOtelCollector'
$CWAServiceDisplayName = 'Amazon CloudWatch Agent'
$CWOCServiceDisplayName = 'CWAgent Otel Collector'
$CWADirectory = 'Amazon\AmazonCloudWatchAgent'
$AllConfig = 'all'

$CWAProgramFiles = "${Env:ProgramFiles}\${CWADirectory}"
if ($Env:ProgramData) {
    $CWAProgramData = "${Env:ProgramData}\${CWADirectory}"
} else {
    # Windows 2003
    $CWAProgramData = "${Env:ALLUSERSPROFILE}\Application Data\${CWADirectory}"
}

$CWALogDirectory = "${CWAProgramData}\Logs"

$CWARestartFile ="${CWAProgramData}\restart"
$CWOCRestartFile ="${CWAProgramData}\cwoc-restart"
$VersionFile ="${CWAProgramFiles}\CWAGENT_VERSION"
$CVLogFile="${CWALogDirectory}\configuration-validation.log"

# The windows service registration assumes exactly this .toml file path and name
$TOML="${CWAProgramData}\amazon-cloudwatch-agent.toml"
$JSON="${CWAProgramData}\amazon-cloudwatch-agent.json"
$JSON_DIR = "${CWAProgramData}\Configs"
$YAML="${CWAProgramData}\${CWOCServiceName}\cwagent-otel-collector.yaml"
$YAML_DIR="${CWAProgramData}\${CWOCServiceName}\Configs"
$PREDEFINED_CONFIG_DATA="${CWAProgramData}\${CWOCServiceName}\predefined-config-data"
$COMMON_CONIG="${CWAProgramData}\common-config.toml"
$ENV_CONFIG="${CWAProgramData}\env-config.json"

$EC2 = $false
# WMI is unavailable on Nano, CIM is unavailable on 2003
$CIM = $false

Function StartAll() {
    Write-Output "****** processing cwagent-otel-collector ******"
    AgentStart -service_name $CWOCServiceName -service_display_name $CWOCServiceDisplayName

    Write-Output "`r`n****** processing amazon-cloudwatch-agent ******"
    AgentStart -service_name $CWAServiceName -service_display_name $CWAServiceDisplayName
}

Function AgentStart() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$service_name,
        [Parameter(Mandatory = $true)]
        [string]$service_display_name
    )

    if (${service_name} -eq $CWOCServiceName -And !(Test-Path -LiteralPath "${YAML}")) {
        Write-Output "cwagent-otel-collector will not be started as it has not been configured yet."
        return
    }

    if (${service_name} -eq $CWAServiceName -And !(Test-Path -LiteralPath "${TOML}")) {
        # If cwagent-otel-collector config exists, amazon-cloudwatch-agent default config will be suppressed
        if (Test-Path -LiteralPath "${YAML}") {
            Write-Output "amazon-cloudwatch-agent will not be started as it has not been configured yet."
            return
        }
        Write-Output "Both amazon-cloudwatch-agent and cwagent-otel-collector are not configured. Applying amazon-cloudwatch-agent default configuration."
        $ConfigLocation = 'default'
        CWAConfig -multi_config 'default'
    }

    $svc = Get-Service -Name "${service_name}" -ErrorAction SilentlyContinue
    if (!$svc) {
        $startCommand = "`"${CWAProgramFiles}\start-amazon-cloudwatch-agent.exe`""
        if (${service_name} -eq $CWOCServiceName) {
            $startCommand = "`"${CWAProgramFiles}\cwagent-otel-collector.exe`" --config=${YAML}"
        }
        New-Service -Name "${service_name}" -DisplayName "${service_display_name}" -Description "${service_display_name}" -DependsOn LanmanServer -BinaryPathName "${startCommand}" | Out-Null
        # object returned by New-Service gives errors so retrieve it again
        $svc = Get-Service -Name "${service_name}"
        # Configure the service to restart on crashes. It's unclear how to do this through WMI or CIM interface so using sc.exe
        # Restarts immediately on the first two crashes then gives a 2 second sleep after any subsequent crash.
        & sc.exe failure "${service_name}" reset= 86400 actions= restart/0/restart/0/restart/2000 | Out-Null
        if ($CIM) {
            & sc.exe failureflag "${service_name}" 1 | Out-Null
        }
    }
    if (${service_name} -eq $CWOCServiceName) {
        $svc | Set-Service -StartupType Automatic -PassThru | Start-Service
    } else {
        $svc | Start-Service
    }
    Write-Output "$service_name has been started"
}

Function StopAll() {
    Write-Output "****** processing cwagent-otel-collector ******"
    AgentStop -service_name $CWOCServiceName

    Write-Output "`r`n****** processing amazon-cloudwatch-agent ******"
    AgentStop -service_name $CWAServiceName
}

Function AgentStop() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$service_name
    )
    $svc = Get-Service -Name "${service_name}" -ErrorAction SilentlyContinue

    if ($svc) {
        if (${service_name} -eq $CWOCServiceName) {
            $svc | Set-Service -StartupType Manual -PassThru | Stop-Service
        } else {
            $svc | Stop-Service
        }
    }
    Write-Output "$service_name has been stopped"
}

Function PrepRestartAll() {
    AgentPrepRestart -service_name $CWOCServiceName -restart_file $CWOCRestartFile
    AgentPrepRestart -service_name $CWAServiceName -restart_file $CWARestartFile
}

Function AgentPrepRestart() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$restart_file,
        [Parameter(Mandatory = $true)]
        [string]$service_name
    )
    if ((Runstatus -service_name $service_name) -eq 'running') {
        Write-Output $null > $restart_file
    }
}

Function CondRestartAll() {
    AgentCondRestart -service_name $CWOCServiceName -service_display_name $CWOCServiceDisplayName -restart_file $CWOCRestartFile
    AgentCondRestart -service_name $CWAServiceName -service_display_name $CWAServiceDisplayName -restart_file $CWARestartFile
}

Function AgentCondRestart() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$restart_file,
        [Parameter(Mandatory = $true)]
        [string]$service_name,
        [Parameter(Mandatory = $true)]
        [string]$service_display_name
    )
    if (Test-Path -LiteralPath "${restart_file}") {
        AgentStart -service_name $service_name -service_display_name $service_display_name
        Remove-Item -LiteralPath "${restart_file}"
    }
}

Function PreunAll() {
    AgentPreun -service_name $CWOCServiceName
    AgentPreun -service_name $CWAServiceName
}

Function AgentPreun() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$service_name
    )

    AgentStop -service_name $service_name
    if ($CIM) {
        $svc = Get-CimInstance -ClassName Win32_Service -Filter "name='${service_name}'"
        $svc | Invoke-CimMethod -MethodName 'delete' | Out-Null
    } else {
        $svc = Get-WmiObject -Class Win32_Service -Filter "name='${service_name}'"
        $svc.delete() | Out-Null
    }
}

Function StatusAll() {
    $cwa_status = Runstatus -service_name ${CWAServiceName}
    $cwa_starttime = GetStarttime -service_name ${CWAServiceName}
    $cwa_config_status = 'configured'
    if (!(Test-Path -LiteralPath "${TOML}")) {
        $cwa_config_status = 'not configured'
    }

    $cwoc_status = Runstatus -service_name ${CWOCServiceName}
    $cwoc_starttime = GetStarttime -service_name ${CWOCServiceName}
    $cwoc_config_status = 'configured'
    if (!(Test-Path -LiteralPath "${YAML}")) {
        $cwoc_config_status = 'not configured'
    }

    $version = ([IO.File]::ReadAllText("${VersionFile}")).Trim()

    Write-Output "{"
    Write-Output "  `"status`": `"${cwa_status}`","
    Write-Output "  `"starttime`": `"${cwa_starttime}`","
    Write-Output "  `"configstatus`": `"${cwa_config_status}`","
    Write-Output "  `"cwoc_status`": `"${cwoc_status}`","
    Write-Output "  `"cwoc_starttime`": `"${cwoc_starttime}`","
    Write-Output "  `"cwoc_configstatus`": `"${cwoc_config_status}`","
    Write-Output "  `"version`": `"${version}`""
    Write-Output "}"
}

Function GetStarttime() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$service_name
    )

    $timefmt=''

    if ($CIM) {
        $svc = Get-CimInstance -ClassName Win32_Service -Filter "name='${service_name}'"
    } else {
        $svc = Get-WmiObject -Class Win32_Service -Filter "name='${service_name}'"
    }

    if ($svc) {
        $agentPid = $svc.ProcessId
        $process = Get-Process -Id "${agentPid}"
        $processStart = $process.StartTime
        if ($processStart) {
            $timefmt = Get-Date -Date ${processStart} -Format "s"
        }
    }

    return $timefmt
}

# Translate platform status names to those used across all CWAgent's platforms
Function Runstatus() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$service_name
    )

    $running = $false
    $svc = Get-Service -Name "${service_name}" -ErrorAction SilentlyContinue
    if ($svc -and ($svc.Status -eq 'running')) {
        $running = $true
    }
    if ($running) {
        return 'running'
    } else {
        return 'stopped'
    }
}

Function ConfigAll() {
    Param (
        [Parameter(Mandatory = $false)]
        [string]$multi_config = 'default'
    )

    if ($OtelConfigLocation) {
        Write-Output "****** processing cwagent-otel-collector ******"
        CWOCConfig -multi_config ${multi_config}
        Write-Output ""
    }

    # cwa_config is called after cwoc_config becuase that whether applying
    # default cwa config depends on the existence of cwoc config
    if ($ConfigLocation) {
        Write-Output "****** processing amazon-cloudwatch-agent ******"
        CWAConfig -multi_config ${multi_config}
    }
}

Function CWAConfig() {
    Param (
        [Parameter(Mandatory = $false)]
        [string]$multi_config = 'default'
    )

    $param_mode="ec2"
    if (!$EC2) {
        $param_mode="onPremise"
    }

    if ($ConfigLocation -eq $AllConfig -And $multi_config -ne 'remove') {
        Write-Output "ignore cwa configuration `"$AllConfig`" as it is only supported by action `"remove-config`""
        return
    }

    if ($ConfigLocation -eq $AllConfig) {
        Remove-Item -Path "${JSON_DIR}\*" -Force -ErrorAction SilentlyContinue
    } else {
        & $CWAProgramFiles\config-downloader.exe --output-dir "${JSON_DIR}" --download-source "${ConfigLocation}" --mode "${param_mode}" --config "${COMMON_CONIG}" --multi-config "${multi_config}"
        CheckCMDResult
    }

    $jsonDirContent = Get-ChildItem "${JSON_DIR}" | Measure-Object

    if ($jsonDirContent.count -eq 0) {
        Write-Output "all amazon-cloudwatch-agent configurations have been removed"
        Remove-Item "${TOML}" -Force -ErrorAction SilentlyContinue
    } else {
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
    }

    if ($Start) {
        AgentStop -service_name $CWAServiceName
        AgentStart -service_name $CWAServiceName -service_display_name $CWAServiceDisplayName
    }
}

Function CWOCConfig() {
    Param (
        [Parameter(Mandatory = $false)]
        [string]$multi_config = 'default'
    )

    if (Test-Path -LiteralPath "${Env:ProgramFiles}\Amazon\AWSOTelCollector\aws-otel-collector-ctl.ps1") {
        Write-Output "REMINDER: you are configuring `"cwagent-otel-collector`" instead of `"aws-otel-collector`"."
    }

    if ($multi_config -eq 'append') {
        Write-Output "ignore `"-o`" as cwagent-otel-collector doesn't support append-config"
        return
    }

    $param_mode="ec2"
    if (!$EC2) {
        $param_mode="onPremise"
    }

    if ($OtelConfigLocation -eq $AllConfig -And $multi_config -ne 'remove') {
        Write-Output "ignore cwoc configuration `"$AllConfig`" as it is only supported by action `"remove-config`""
        return
    }

    if ($OtelConfigLocation -eq $AllConfig) {
        Remove-Item -Path "${YAML_DIR}\*" -Force -ErrorAction SilentlyContinue
    } elseif ($multi_config -eq 'default' -And $OtelConfigLocation -eq 'default') {
        Copy-Item "${PREDEFINED_CONFIG_DATA}" -Destination "${YAML_DIR}/default.tmp"
        Write-Output "Successfully fetched the config and saved in ${YAML_DIR}\default.tmp"
    } else {
        & $CWAProgramFiles\config-downloader.exe --output-dir "${YAML_DIR}" --download-source "${OtelConfigLocation}" --mode "${param_mode}" --config "${COMMON_CONIG}" --multi-config "${multi_config}"
        if ($LASTEXITCODE -ne 0) {
            return
        }
    }

    $yamlDirContent = Get-ChildItem "${YAML_DIR}" | Measure-Object
    if ($yamlDirContent.count -eq 0) {
        Write-Output "all cwagent-otel-collector configurations have been removed"
        Remove-Item "${YAML}" -Force -ErrorAction SilentlyContinue
    } else {
        # delete old file which are without .tmp suffix
        Remove-Item -Path "${YAML_DIR}\*" -Exclude "*.tmp" -Force -ErrorAction SilentlyContinue

        # trip the .tmp suffix from new file, and copy it to the YAML
        Get-ChildItem "${YAML_DIR}" | ForEach-Object {
            $newName = $_.name -Replace  '\.tmp$',''
            $destination = Join-Path -Path $_.Directory.FullName -ChildPath "${newName}"
            Move-Item $_.FullName -Destination "${destination}" -Force
            Copy-Item "${destination}" -Destination "${YAML}" -Force
            Write-Output "cwagent-otel-collector config has been successfully fetched."
        }
    }

    if ($Start) {
        AgentStop -service_name $CWOCServiceName
        AgentStart -service_name $CWOCServiceName -service_display_name $CWOCServiceDisplayName
    }
}

# For exes(non cmlet) the $ErrorActionPreference won't help if run cmd result failed,
# We have to check the $LASTEXITCODE everytime.
Function CheckCMDResult($ErrorMessage, $SuccessMessage) {
    if ($LASTEXITCODE -ne 0) {
        if (![string]::IsNullOrEmpty($ErrorMessage)) {
            Write-Output $ErrorMessage
        }
        exit 1
    } else {
        if (![string]::IsNullOrEmpty($SuccessMessage)) {
            Write-Output $SuccessMessage
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

Function SetLogLevelAll() {
    switch -exact ($LogLevel) {
        INFO { }
        DEBUG { }
        ERROR { }
        WARN { }
        OFF { }
        default {
            Write-Output "Invalid log level: ${LogLevel}`n${UsageString}"
            Exit 1
        }
    }

    & cmd /c "`"${CWAProgramFiles}\amazon-cloudwatch-agent.exe`" --setenv CWAGENT_LOG_LEVEL=${LogLevel} --envconfig ${ENV_CONFIG} 2>&1"
    CheckCMDResult "" "Set CWAGENT_LOG_LEVEL to ${LogLevel}"
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
        stop { StopAll }
        start { StartAll }
        fetch-config { ConfigAll }
        append-config { ConfigAll -multi_config 'append' }
        remove-config { ConfigAll -multi_config 'remove' }
        status { StatusAll }
        prep-restart { PrepRestartAll }
        cond-restart { CondRestartAll }
        preun { PreunAll }
        set-log-level { SetLogLevelAll }
        default {
           Write-Output "Invalid action: ${Action}`n${UsageString}"
           Exit 1
        }
    }
}

main
