Function assertStatus() {
    Param (
        [Parameter(Mandatory = $true)]
        [string]$keyToCheck,
        [Parameter(Mandatory = $true)]
        [string]$expectedVal
    )

    $interestedKey = 'unknown'
    switch -exact ($keyToCheck) {
        cwa_running_status { $interestedKey = "status" }
        cwa_config_status { $interestedKey = "configstatus" }
        cwoc_running_status { $interestedKey = "cwoc_status" }
        cwoc_config_status { $interestedKey = "cwoc_configstatus" }
        default {
           Write-Output "Invalid keyToCheck: $keyToCheck"
           Exit 1
        }
    }

	$output = & "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a status | ConvertFrom-Json

	foreach ($jsonBlob in $output) {
	    $result = $($jsonBlob.$interestedKey)
	    if ( $result -eq $expectedVal ) {
	        Write-Output "In step ${step}, ${keyToCheck} is expected"
	    } else {
	        Write-Output "In step ${step}, ${keyToCheck} is NOT expected. (actual=`"${result}`"; expected=`"${expectedVal}`")"
	        exit 1
	    }
    }

}

# init
$step=0
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c all -o all
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop

$step=1
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a status
assertStatus -keyToCheck "cwa_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "not configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "not configured"

$step=2
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a start
assertStatus -keyToCheck "cwa_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "not configured"

$step=3
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a fetch-config -o default -s
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c default -s
assertStatus -keyToCheck "cwa_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "not configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "configured"

$step=4
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a fetch-config -c default -o invalid -s
assertStatus -keyToCheck "cwa_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "configured"

$step=5
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a prep-restart
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop
assertStatus -keyToCheck "cwa_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "configured"

$step=6
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a cond-restart
assertStatus -keyToCheck "cwa_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "configured"

$step=7
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c default -s
assertStatus -keyToCheck "cwa_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "not configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "configured"

$step=8
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -o default -s
assertStatus -keyToCheck "cwa_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "not configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "not configured"

$step=9
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a append-config -c default -o default -s
assertStatus -keyToCheck "cwa_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "not configured"

$step=10
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a remove-config -c all
assertStatus -keyToCheck "cwa_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "not configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "not configured"

$step=11
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a fetch-config -o default -s
assertStatus -keyToCheck "cwa_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "running"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "not configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "configured"

$step=12
& "C:\Program Files\Amazon\AmazonCloudWatchAgent\amazon-cloudwatch-agent-ctl.ps1" -a stop
assertStatus -keyToCheck "cwa_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwoc_running_status" -expectedVal "stopped"
assertStatus -keyToCheck "cwa_config_status" -expectedVal "not configured"
assertStatus -keyToCheck "cwoc_config_status" -expectedVal "configured"