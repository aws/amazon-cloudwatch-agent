# CloudWatch Agent

`fetch-config` replaces the agent configuration. After a successful replace it deletes the legacy file `amazon-cloudwatch-agent.json`, then promotes the downloaded `*.tmp` files in the config directory.

Do not delete that legacy file when `-c file:` points at it. That path is a supported config source, and deleting it removes the file the operator just applied. SSM parameters, `default`, and any other file path still delete it. Agent start merges the legacy json with the files under the config directory, so leaving a stale copy in place would apply both.

Keep this check in all three control scripts:

- `packaging/dependencies/amazon-cloudwatch-agent-ctl`
- `packaging/darwin/amazon-cloudwatch-agent-ctl`
- `packaging/windows/amazon-cloudwatch-agent-ctl.ps1`

Set `AMAZON_CLOUDWATCH_AGENT_CTL_SOURCE_ONLY=1` to source a control script without running `main`. The regression test is:

```
go test ./packaging/dependencies -count=1 -run TestFetchConfigKeepsLegacyJSONSource
```
