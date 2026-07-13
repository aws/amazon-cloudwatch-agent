# Tomcat Process Detector

Detects and classifies Tomcat processes running on the system. 

> [!WARNING]
> This is meant to be used as a sub-detector of the [Java Process Detector](../java) and will result in incomplete metadata
> results if used on its own.

## Overview

The Tomcat detector identifies Tomcat instances by searching for:
- `CATALINA_BASE`: Runtime configuration directory (per-instance, preferred)
- `CATALINA_HOME`: Installation directory (commonly shared, fallback)

It checks both system properties and environment variables in the following **priority order**:
1. Catalina Base System Property (`-Dcatalina.base`)
2. Catalina Base Environment Variable (`CATALINA_BASE`)
3. Catalina Home System Property (`-Dcatalina.home`)
4. Catalina Home Environment Variable (`CATALINA_HOME`)

The detected directory is used as the metadata name.

### Sample Metadata Result
```json
{
  "categories": ["TOMCAT"],
  "name": "/opt/tomcat/instance1"
}
```
