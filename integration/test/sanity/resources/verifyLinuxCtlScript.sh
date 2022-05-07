#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

assertStatus() {
     keyToCheck="${1:-}"
     expectedVal="${2:-}"

     grepKey='unknown'
     case "${keyToCheck}" in
     cwa_running_status)
          grepKey="\"status\""
          ;;
     cwa_config_status)
          grepKey="\"configstatus\""
          ;;
     cwoc_running_status)
          grepKey="\"cwoc_status\""
          ;;
     cwoc_config_status)
          grepKey="\"cwoc_configstatus\""
          ;;
     *)
          echo "Invalid Key To Check: ${keyToCheck}" >&2
          exit 1
          ;;
     esac

     result=$(/usr/bin/amazon-cloudwatch-agent-ctl -a status | grep "${grepKey}" | awk -F: '{print $2}' | sed 's/ "//; s/",//')

     if [ "${result}" = "${expectedVal}" ]; then
          echo "In step ${step}, ${keyToCheck} is expected"
     else
          echo "In step ${step}, ${keyToCheck} is NOT expected. (actual="${result}"; expected="${expectedVal}")"
          exit 1
     fi
}

# init
step=0
aoc_user=$(cat /etc/passwd | grep aoc)
if [ "${aoc_user}" = "" ]; then
     echo 'aoc:x:995:991:AOC Agent:/home/aoc:/sbin/nologin' | sudo tee -a /etc/passwd
fi
/usr/bin/amazon-cloudwatch-agent-ctl -a remove-config -c all -o all
/usr/bin/amazon-cloudwatch-agent-ctl -a stop

step=1
/usr/bin/amazon-cloudwatch-agent-ctl -a status
assertStatus "cwa_running_status" "stopped"
assertStatus "cwoc_running_status" "stopped"
assertStatus "cwa_config_status" "not configured"
assertStatus "cwoc_config_status" "not configured"

step=2
/usr/bin/amazon-cloudwatch-agent-ctl -a start
assertStatus "cwa_running_status" "running"
assertStatus "cwoc_running_status" "stopped"
assertStatus "cwa_config_status" "configured"
assertStatus "cwoc_config_status" "not configured"

step=3
/usr/bin/amazon-cloudwatch-agent-ctl -a fetch-config -o default -s
/usr/bin/amazon-cloudwatch-agent-ctl -a remove-config -c default -s
assertStatus "cwa_running_status" "stopped"
assertStatus "cwoc_running_status" "running"
assertStatus "cwa_config_status" "not configured"
assertStatus "cwoc_config_status" "configured"

step=4
/usr/bin/amazon-cloudwatch-agent-ctl -a fetch-config -c default -o invalid -s
assertStatus "cwa_running_status" "running"
assertStatus "cwoc_running_status" "running"
assertStatus "cwa_config_status" "configured"
assertStatus "cwoc_config_status" "configured"

step=5
/usr/bin/amazon-cloudwatch-agent-ctl -a prep-restart
/usr/bin/amazon-cloudwatch-agent-ctl -a stop
assertStatus "cwa_running_status" "stopped"
assertStatus "cwoc_running_status" "stopped"
assertStatus "cwa_config_status" "configured"
assertStatus "cwoc_config_status" "configured"

step=6
/usr/bin/amazon-cloudwatch-agent-ctl -a cond-restart
assertStatus "cwa_running_status" "running"
assertStatus "cwoc_running_status" "running"
assertStatus "cwa_config_status" "configured"
assertStatus "cwoc_config_status" "configured"

step=7
/usr/bin/amazon-cloudwatch-agent-ctl -a remove-config -c default -s
assertStatus "cwa_running_status" "stopped"
assertStatus "cwoc_running_status" "running"
assertStatus "cwa_config_status" "not configured"
assertStatus "cwoc_config_status" "configured"

step=8
/usr/bin/amazon-cloudwatch-agent-ctl -a remove-config -o default -s
assertStatus "cwa_running_status" "stopped"
assertStatus "cwoc_running_status" "stopped"
assertStatus "cwa_config_status" "not configured"
assertStatus "cwoc_config_status" "not configured"

step=9
/usr/bin/amazon-cloudwatch-agent-ctl -a append-config -c default -o default -s
assertStatus "cwa_running_status" "running"
assertStatus "cwoc_running_status" "stopped"
assertStatus "cwa_config_status" "configured"
assertStatus "cwoc_config_status" "not configured"

step=10
/usr/bin/amazon-cloudwatch-agent-ctl -a remove-config -c all
assertStatus "cwa_running_status" "running"
assertStatus "cwoc_running_status" "stopped"
assertStatus "cwa_config_status" "not configured"
assertStatus "cwoc_config_status" "not configured"

step=11
/usr/bin/amazon-cloudwatch-agent-ctl -a fetch-config -o default -s
assertStatus "cwa_running_status" "running"
assertStatus "cwoc_running_status" "running"
assertStatus "cwa_config_status" "not configured"
assertStatus "cwoc_config_status" "configured"

step=12
/usr/bin/amazon-cloudwatch-agent-ctl -a stop
assertStatus "cwa_running_status" "stopped"
assertStatus "cwoc_running_status" "stopped"
assertStatus "cwa_config_status" "not configured"
assertStatus "cwoc_config_status" "configured"
