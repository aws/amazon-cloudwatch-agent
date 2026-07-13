# Java Process Detector

Detects and classifies Java processes running on the system.

## Overview
The Java detector evaluates each process using the following steps:
1. Verifies the process is running Java (`java` executable)
2. Attempts application-specific detection using sub-detectors
3. Always prepends the `JVM` category to each metadata result
4. If the sub-detectors did not match or did not extract a metadata name, falls back to generic Java name extraction
5. Attempts to extract the JMX port to determine metadata status
6. Returns the metadata result

### Sub-detectors
- [Tomcat Process Detector](../tomcat)

### Generic Java Process Name Extraction
1. If Java archive (`.jar`, `.war`):
   1. Attempts to extract the names from the manifest file in the following **priority order**:
      1. `Application-Name` - Non-standard, but explicit name
      2. `Implementation-Title` - Standard application name
      3. `Start-Class` - Spring Boot application class (not the launcher)
      4. `Main-Class` - Entry point class (fallback)
   2. If none of those are found, falls back on the archive base filename without the extension (e.g. `app.jar -> app`)
2. If not Java archive:
   1. Falls back on the entrypoint class name from the command-line

### JMX Port Detection
Extracts JMX ports from command line arguments or environment variables in the following **priority order**:
1. System Property (`-Dcom.sun.management.jmxremote.port`)
2. Environment Variable (`JMX_PORT`)

### Sample Metadata Result
```json
{
  "categories": ["JVM"],
  "name": "com.example.DemoApplication",
  "telemetry_port": 1234,
  "status": "READY"
}
```
