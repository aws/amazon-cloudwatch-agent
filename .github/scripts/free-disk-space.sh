#!/bin/bash
set -e

echo "=== Disk usage before cleanup ==="
df -h

echo "=== Freeing up disk space ==="
sudo rm -rf /usr/share/dotnet
sudo rm -rf /usr/local/share/powershell
sudo rm -rf /usr/local/lib/android
sudo rm -rf /opt/ghc
sudo rm -rf "$AGENT_TOOLSDIRECTORY"

echo "=== Disk usage after cleanup ==="
df -h
