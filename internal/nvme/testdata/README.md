# Test Data for Instance Store NVMe Receiver

This directory contains test data files used for testing the Instance Store NVMe receiver functionality.

## Files

- `sample_instance_store_log_page.bin`: A valid Instance Store log page with the correct magic number (0xEC2C0D7E) and sample metric values
- `invalid_magic_log_page.bin`: A log page with an invalid magic number for testing error handling
- `minimal_log_page.bin`: A minimal 96-byte log page with zero values for testing edge cases

## Usage

These binary files are used by the unit tests to validate log page parsing functionality without requiring actual NVMe devices.