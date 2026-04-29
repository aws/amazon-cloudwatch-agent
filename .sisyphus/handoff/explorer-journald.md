# Journald Code Exploration — CloudWatch Agent

**Repo**: `/workplace/mcommey/cloudwatch-agent/amazon-cloudwatch-agent`
**Current branch**: `mcommey/journald-logs-support` (based on mainline, does NOT contain journald commits yet)

## Git Branches with Journald Work

| Branch | Key Commits | Status |
|--------|------------|--------|
| `remotes/origin/feature/journald-logs-support` | `1ccf2494` feat: Add comprehensive journald logs support | Older base (pre-1.300067.0) |
| `remotes/origin/mpmann/journald-receiver` | `89a21ee9`, `6dc78837`, `cb662731` — receiver dep, pipeline integration, docs | More recent, has OTel integration |

**IMPORTANT**: Neither remote branch's journald commits are merged into the current local branch HEAD (`693f2e41`). The local branch appears to be freshly created from mainline without journald work.

## Files on Disk (current branch)

Despite commits not being ancestors, these files exist on disk (likely from a prior merge/cherry-pick or manual creation):

### 1. Translator Code (OTel pipeline)
- `translator/translate/otel/receiver/journald/translator.go` — Journald receiver translator
- `translator/translate/otel/receiver/journald/translator_test.go`
- `translator/translate/otel/receiver/journald/testdata/` — basic_config.json, multiple_collect_list.json, multiple_units.json, with_filters.json
- `translator/translate/otel/exporter/journald/translator.go` — CWL exporter for journald pipelines
- `translator/translate/otel/exporter/journald/translator_test.go`
- `translator/translate/otel/processor/journaldfilter/translator.go` — Filter processor for journald
- `translator/translate/otel/processor/journaldfilter/translator_test.go`
- `translator/translate/otel/pipeline/journald/translator.go` — Pipeline orchestrator for journald
- `translator/translate/otel/pipeline/journald/translator_test.go`

### 2. Config/Schema
- `translator/config/schema.json` — Contains journald schema definition
- `translator/config/sampleSchema/validLogJournald.json`
- `translator/config/sampleSchema/validLogJournaldWithFilters.json`
- `translator/config/sampleSchema/invalidLogJournaldWithInvalidFilterType.json`
- `translator/config/sampleSchema/invalidLogJournaldWithInvalidLogStreamNameType.json`
- `translator/config/sampleSchema/invalidLogJournaldWithInvalidUnitsType.json`

### 3. Tool/Wizard Code (config generation)
- `tool/data/config/logs/journald.go` (38 lines) — Journald data model
- `tool/data/config/logs/journaldConfig.go` (46 lines) — Journald config struct
- `tool/data/config/logs/journaldConfig_test.go` (71 lines)
- `tool/data/config/logs/journald_test.go` (108 lines)
- `tool/processors/question/journald/journald.go` (141 lines) — Wizard questions for journald
- `tool/processors/question/journald/journald_test.go` (88 lines)

### 4. Sample OTel Configs
- `translator/tocwconfig/sampleConfig/complete_linux_config.yaml` — Contains journald pipeline examples (journald_0, journald_1)
- `translator/tocwconfig/sampleConfig/complete_linux_config.json`
- `translator/tocwconfig/sampleConfig/log_only_config_linux.json`

### 5. Component Registration
- `service/defaultcomponents/components.go:46` — imports `journaldreceiver`
- `service/defaultcomponents/components.go:103` — registers `journaldreceiver.NewFactory()`
- `service/defaultcomponents/components_test.go:31` — tests "journald" receiver

### 6. OTel Pipeline Integration
- `translator/translate/otel/translate_otel.go:34` — imports journald pipeline package
- `translator/translate/otel/translate_otel.go:83` — `translators.Merge(journald.NewTranslators(conf))`
- `translator/translate/otel/common/common.go:85` — `JournaldKey = "journald"`

### 7. Dependency
- `go.mod:257` — `github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver v0.150.0`

## Files in Git History but NOT on Current Branch
- `translator/translate/otel/pipeline/journald/translator_nonlinux.go` (31 lines) — build constraint stub
- `translator/translate/otel/receiver/journald/translator_nonlinux.go` (40 lines) — build constraint stub
- `translator/translate/otel/receiver/journald/README.md` — documentation

## Key Integration Points
1. **Receiver**: OTel `journaldreceiver` from contrib, registered in components factory
2. **Pipeline**: Custom translator creates per-collect_list-entry pipelines (journald_0, journald_1, etc.)
3. **Exporter**: Dedicated CWL exporter translator per journald pipeline
4. **Processor**: `journaldfilter` processor for filtering journal entries
5. **Config key**: `JournaldKey = "journald"` under logs section
6. **Schema**: JSON schema validates journald config with units, filters, log_stream_name
