# packaging/ — OS-Specific Packaging

## What This Is

Packaging scripts and resources for building distributable packages across all supported platforms.

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `linux/` | Linux packaging (systemd unit files, init scripts, install/uninstall scripts) |
| `darwin/` | macOS packaging (launchd plist, install scripts) |
| `debian/` | Debian/Ubuntu `.deb` package metadata (control file, postinst, prerm) |
| `windows/` | Windows packaging (MSI resources, Windows Service registration) |
| `dependencies/` | Shared dependency declarations |

## Also Contains

- `opentelemetry-jmx-metrics.jar` — JMX metrics collection JAR bundled with the agent.
- `update-jmx-jar.sh` — Script to update the bundled JMX JAR.

## Key Patterns

- The `Tools/src/` directory (at repo root) contains the actual package build scripts (`create_rpm.sh`, `create_deb.sh`, `create_win.sh`, `create_darwin.sh`).
- Linux packages install to `/opt/aws/amazon-cloudwatch-agent/`.
- The agent runs as a systemd service on Linux, a Windows Service on Windows, and a launchd daemon on macOS.
