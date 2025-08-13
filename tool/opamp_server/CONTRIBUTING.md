# Contributing to opamp-go library

Welcome, and thank you for your interest in contributing to the opamp-go library!

Whether you're submitting a bug fix, implementing a new feature, or updating documentation, we appreciate your effort in making opamp-go better! If you're new to the project, don’t hesitate to ask for help — we value collaboration and learning.

Before getting started, please ensure you’re familiar with the contribution process by reviewing this guide. Also, check out the [OpenTelemetry contributing guide](https://github.com/open-telemetry/community/blob/main/guides/contributor/README.md) for general contribution guidelines.

## Pre-requisites

* Ensure you have Go installed (check the required version in [go.mod](https://github.com/open-telemetry/opamp-go/blob/main/go.mod)).

* Install Docker, which is required for generating protobuf files.

## Generate Protobuf Go Files

To set up and run opamp-go locally, follow these steps:

Clone the repository with submodules:

```
git clone --recurse-submodules git@github.com:open-telemetry/opamp-go.git
cd opamp-go
```

If you've already cloned the repository without submodules, initialize them with:

```
git submodule update --init
```

This will fetch all files for `opamp-spec` submodule. 
Note that `opamp-spec` submodule requires ssh git cloning access to github and won't work if you only have https access.

Generate the protobuf Go files:

```
make gen-proto
```
This should compile `internal/proto/*.proto` files to `internal/protobufs/*.pb.go` files.

Note that this is tested on linux/amd64 and darwin/amd64 and is known to not work on darwin/arm64 (M1/M2 Macs). Fixes are welcome.

# Releasing a new version of opamp-go

1. `Draft a new release` on the releases page in GitHub.

2. Create a new tag for the release and target the `main` branch.

3. Use the `Generate release notes` button to automatically generate release notes. Modify as appropriate.

4. Check `Set as a pre-release` as appropriate.

5. `Publish release`

## Further Help

If you have any questions or run into issues while contributing, feel free to reach out to the OpenTelemetry community:

* Slack: Join the CNCF [#otel-opamp](https://cloud-native.slack.com/archives/C02J58HR58R) Slack channel to connect with other contributors and get access to SIG meeting notes
  
* GitHub Issues & Discussions: Report bugs or propose features via [GitHub Issues](https://github.com/open-telemetry/opamp-go/issues).

We’re excited to have you contribute and look forward to your ideas and improvements! 🚀
