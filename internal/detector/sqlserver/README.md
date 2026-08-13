# SQL Server Process Detector

Detects SQL Server database server processes running on the system.

## Overview

The SQL Server detector identifies SQL Server instances by checking for the `sqlservr` executable name. It is used by the `workload-discovery` command to automatically discover SQL Server workloads.

## Detection Method

The detector examines the executable path of each process and checks if the base name is `sqlservr`.

## Status Results

- `READY`: SQL Server process detected with a port (explicit via `-p` flag or `MSSQL_TCP_PORT`, otherwise defaults to 1433).

## Port Detection

The detector attempts to find the SQL Server port using the following priority:
1. **Windows only**: Windows Registry (resolves named instance ports automatically)
2. Command-line flags: `-p`
3. Environment variable: `MSSQL_TCP_PORT`
4. Default fallback: 1433

Invalid or malformed port values (e.g., `-p 0`, `-p 99999`, `-p abc`) are rejected and the detector falls back to the default port.

## Sample Metadata Result

```json
{
  "categories": ["SQLSERVER"],
  "name": "sqlserver",
  "status": "READY",
  "telemetry_port": 1433
}
```
