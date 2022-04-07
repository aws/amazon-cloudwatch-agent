#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

n="$1"
if [ -z "$1" ] || [ $n -le 1 ]; then
  n=100
fi
file_path="$2"
if [ -z "$2" ]; then
  file_path='/tmp/test.log'
fi

echo "Emitting $n logs to $file_path"

for i in $(seq 1 $n); do
  echo "$(date -Is) [foo] This is a log line" >>$file_path
  echo "$(date -Is) [bar] This is a log line" >>$file_path
  sleep 1 # sleep so the timestamps for the logs are different
done
