# MySQL Process Detector

Detects MySQL database server processes running on the system.

## Overview

The MySQL detector identifies MySQL server instances by checking for the `mysqld` executable name. It is used by the `workload-discovery` command to automatically discover MySQL workloads.

## Detection Method

The detector examines the executable path of each process and checks if the base name is `mysqld`.

## Port Detection

The detector attempts to find the MySQL port using the following priority:
1. Command-line flags: `--port` or `-P`
2. Environment variable: `MYSQL_TCP_PORT`
3. Default fallback: 3306

Invalid or malformed port values (e.g., `--port=0`, `--port=99999`, `--port=abc`) are rejected and the detector falls back to the default port.

## Status Results
- `READY`: MySQL process detected with a port (explicit via `--port`/`-P` flag or `MYSQL_TCP_PORT`, otherwise defaults to 3306).

## Sample Metadata Result
```json
{
  "categories": ["MYSQL"],
  "name": "mysql",
  "status": "READY",
  "telemetry_port": 3306
}
```
