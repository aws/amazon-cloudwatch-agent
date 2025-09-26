# Kafka Process Detector

Detects and classifies Kafka processes running on the system.

> [!NOTE]
> This detector currently only supports detection of Kafka brokers.

> [!WARNING]
> This is meant to be used as a sub-detector of the [Java Process Detector](../java) and will result in incomplete metadata
> results if used on its own.

## Overview

The Kafka detector identifies Kafka broker instances by searching for the Kafka main class (`kafka.Kafka`) in command-line arguments.

### Sample Metadata Result
```json
{
  "categories": ["Kafka/Broker"],
  "name": "Kafka Broker"
}
```
