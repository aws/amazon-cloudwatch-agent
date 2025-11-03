# Kafka Broker Process Detector

Detects and classifies Kafka Broker processes running on the system.

> [!WARNING]
> This is meant to be used as a sub-detector of the [Java Process Detector](../java) and will result in incomplete metadata
> results if used on its own.

## Overview

The Kafka detector identifies Kafka broker instances by searching for the Kafka main class (`kafka.Kafka`) in command-line arguments.
It extracts the broker configuration and identifying information from multiple sources.

### Extracted Attributes

The detector extracts the following attributes when available:
- `broker.id` - Unique identifier for the Kafka broker (normalized from `node.id` in KRaft mode)
- `cluster.id` - Unique identifier for the Kafka cluster

### Configuration Sources (in priority order)

1. Meta properties file in Kafka log directories
2. Command line `--override` arguments
3. Server properties file

### Sample Metadata Result
```json
{
  "categories": ["KAFKA/BROKER"],
  "name": "Kafka Broker",
  "attributes": {
    "broker.id": "0",
    "cluster.id": "WQSzAfd_RvO0TocjqhQoaA"
  }
}
```
