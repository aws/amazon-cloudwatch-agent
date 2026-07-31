# PostgreSQL Process Detector

Detects PostgreSQL database server processes running on the system.

## Overview

The PostgreSQL detector identifies PostgreSQL server instances by checking for the `postgres` executable name. It is used by the `workload-discovery` command to automatically discover PostgreSQL workloads.

## Detection Method

The detector examines the executable path of each process and checks if the base name is `postgres`.

## Status Results
- `READY`: PostgreSQL process detected with a port (explicit via `-p` flag or `PGPORT`, otherwise defaults to 5432).

## Sample Metadata Result
```json
{
  "categories": ["POSTGRESQL"],
  "name": "postgresql",
  "status": "READY",
  "telemetryPort": 5432
}
```
