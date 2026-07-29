# MySQL Process Detector

Detects MySQL database server processes running on the system.

## Overview

The MySQL detector identifies MySQL server instances by checking for the `mysqld` executable name. It is used by the `workload-discovery` command to automatically discover MySQL workloads.

## Detection Method

The detector examines the executable path of each process and checks if the base name is `mysqld`.

## Status Results
- `READY`: MySQL process detected with a port (explicit via `--port`/`-P` flag or `MYSQL_TCP_PORT`, otherwise defaults to 3306).

## Sample Metadata Result
```json
{
  "categories": ["MYSQL"],
  "name": "mysql",
  "status": "READY",
  "telemetryPort": 3306
}
```
