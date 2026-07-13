# NVIDIA GPU Device Detector

Detects NVIDIA GPU devices and driver status on the system.

## Overview

The NVIDIA detector identifies NVIDIA GPU hardware and driver availability using platform-specific checks:

### Linux Detection
- `/dev/nvidia[0-9]+` device files for hardware detection
- `nvidia-smi` utility presence for driver verification

### Windows Detection
- Windows registry check for VEN_10DE (NVIDIA vendor ID) for hardware detection
- `nvidia-smi.exe` utility presence for driver verification

## Status Results
- `READY`: NVIDIA devices detected and drivers are loaded
- `NEEDS_SETUP/NVIDIA_DRIVER`: NVIDIA devices detected but drivers are not loaded
- Returns `ErrIncompatibleDetector` if no NVIDIA devices are found

### Sample Metadata Result
```json
{
  "categories": ["NVIDIA_GPU"],
  "status": "READY"
}
```
