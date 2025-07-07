# Sample Log File for CloudWatch Agent Testing

This directory contains a sample log file (`sample_logs.log`) with log entries of various sizes to test CloudWatch Agent behavior with different log entry sizes.

## Log Entry Sizes:

1. **Small logs (< 256 KiB)**: Regular application logs (~200 bytes each)
2. **Exactly 256 KiB**: 262,144 bytes (262,143 bytes + newline)
3. **Exactly 258 KiB**: 264,192 bytes (264,191 bytes + newline) 
4. **Large log (> 256 KiB)**: 300 KiB (307,200 bytes)

## File Details:
- Total file size: ~815 KB
- Contains realistic log entries with timestamps, log levels, and contextual information
- Large entries simulate scenarios like memory dumps, bulk operations, and error traces

## Usage:
This file can be used to test how CloudWatch Agent handles log entries that exceed the typical 256 KiB limit, which is a common constraint in log processing systems.

## CloudWatch Agent Considerations:
- CloudWatch Logs has a maximum log event size of 256 KB
- Log entries larger than 256 KB may be truncated or rejected
- This test file helps verify the agent's behavior with various log sizes
