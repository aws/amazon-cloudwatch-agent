# SQL Server Process Detector

Detects SQL Server database server processes running on the system.

## Overview

The SQL Server detector identifies SQL Server instances by checking for the `sqlservr` executable name. It is used by the `workload-discovery` command to automatically discover SQL Server workloads.

## Detection Method

The detector examines the executable path of each process and checks if the base name is `sqlservr`.

## Status Results

- `READY`: SQL Server process detected with a port (explicit via `-p` flag or `MSSQL_TCP_PORT`, otherwise defaults to 1433).

## Sample Metadata Result

```json
{
  "categories": ["SQLSERVER"],
  "name": "sqlserver",
  "status": "READY",
  "telemetryPort": 1433
}
```
