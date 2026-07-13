# Kafka Client Process Detector

Detects and classifies Kafka Client processes running on the system.

> [!NOTE]
> This detector only supports detection of processes that use the Kafka clients library.

> [!WARNING]
> This is meant to be used as a sub-detector of the [Java Process Detector](../java) and will result in incomplete metadata
> results if used on its own.

## Overview

The Kafka client detector identifies Java applications that use the Kafka client library (`kafka-clients`) by searching in:
1. Command line classpath (`-cp`/`-classpath`) arguments
2. Open file descriptors for `kafka-clients.jar`

### Sample Metadata Result
```json
{
  "categories": ["KAFKA/CLIENT"]
}
```
