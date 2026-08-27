# AI Gateway Testkit

[![CI](https://github.com/trungdlp/ai-gateway-testkit/actions/workflows/ci.yml/badge.svg)](https://github.com/trungdlp/ai-gateway-testkit/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

AI Gateway Testkit (`agtk`) is an open-source conformance and regression runner for OpenAI- and Anthropic-compatible gateways. It produces stable scenario and assertion IDs, versioned compatibility profiles, readiness verdicts, and shareable canonical reports.

It currently covers authentication, model discovery, non-streaming responses/messages, forced tool calls, official Go SDK interoperability, behavioral diagnostics, and optional Codex/Claude Code workflows in ephemeral Docker Sandboxes.

## Why stable IDs matter

Each scenario has a permanent ID such as `OAI-RESP-001`, `ANT-TOOL-001`, or `CDX-EXEC-001`. Its atomic assertions are addressed as `OAI-RESP-001/A04`. A team can therefore discuss, baseline, waive, and compare the same failure without interpreting log prose.

Compatibility claims are explicit profiles. A gateway can pass `oai-core`, fail `anthropic-tools`, and have `codex-ready` remain `INDETERMINATE` when operational evidence was not collected.

## Install

Go 1.25 or newer is required.

```sh
go install github.com/trungdlp/ai-gateway-testkit/cmd/agtk@latest
```

## Quick start

The legacy single-endpoint interface is convenient for local runs:

```sh
export AI_GATEWAY_BASE_URL='https://gateway.example.com/v1'
export AI_GATEWAY_MODEL='your-model-id'
export AI_GATEWAY_PROTOCOL='both'
export AI_GATEWAY_API_KEY='your-api-key'

agtk run
```

For independent protocol configuration, use a target manifest based on [examples/target.yaml](examples/target.yaml):

```sh
export OPENAI_GATEWAY_API_KEY='...'
export ANTHROPIC_GATEWAY_API_KEY='...'

agtk run --target examples/target.yaml \
  --profile oai-tools,oai-sdk-go \
  --profile anthropic-tools,anthropic-sdk-go \
  --format json --output report.json
```

Credentials are referenced by environment-variable name; they are never accepted as flag values or written to reports.

Transient failures are retried twice by default with bounded exponential backoff:

```sh
agtk run --target target.yaml \
  --retries 3 \
  --retry-backoff 250ms \
  --retry-max-wait 5s
```

Retryable conditions are transport failures and HTTP `408`, `425`, `429`, `500`, `502`, `503`, and `504`. `Retry-After` is honored up to `--retry-max-wait`. Stable client/auth/schema failures are never retried. Set `--retries 0` when duplicate inference cost is unacceptable.

## Commands

```sh
agtk profiles
agtk catalog list
agtk catalog show OAI-TOOL-001
agtk catalog validate
agtk catalog export

agtk run --target target.yaml --profile oai-tools
agtk run --target target.yaml --profile codex-ready --agent-runner sbx

agtk compare baseline.json current.json
agtk report sanitize report.json > shareable.json
```

When no profile is selected, `agtk` runs the core, tool, and official SDK profiles applicable to the configured protocol endpoints. Behavioral diagnostics and agent tests are never selected implicitly.

Available profiles include:

- `oai-core`, `oai-tools`, `oai-sdk-go`, `behavioral-openai`;
- `anthropic-core`, `anthropic-tools`, `anthropic-sdk-go`, `behavioral-anthropic`;
- experimental `codex-ready` and `claude-code-ready`.

The generated [test catalog](docs/test-catalog.md) is the current index of scenarios and profiles.

## Verdicts and exit codes

A profile is:

- `PASS` when every required assertion passes;
- `FAIL` when any required assertion produces evidence of incompatibility;
- `INDETERMINATE` when required evidence is missing, errored, blocked, skipped, or not applicable.

Reports separately expose success and coverage ratios so unavailable evidence cannot masquerade as either success or incompatibility.

| Code | Meaning |
| ---: | --- |
| `0` | Every selected profile passed. |
| `1` | At least one selected profile failed or was indeterminate. |
| `2` | Configuration, invocation, catalog, runner, or report I/O failed. |

## Agent workflows

Install and authenticate Docker Sandboxes (`sbx`) before selecting an agent profile. Agent cases use a disposable fixture rather than the current repository, scope a proxy-managed secret to the sandbox and gateway host, run non-interactively, verify filesystem and shell effects, and remove the named sandbox afterward.

```sh
agtk run --target target.yaml --profile codex-ready --agent-runner sbx
agtk run --target target.yaml --profile claude-code-ready --agent-runner sbx
```

Without `--agent-runner sbx`, agent assertions are skipped and their readiness profile is indeterminate.

## Reports and security

The canonical JSON schema is [schemas/report.schema.json](schemas/report.schema.json). Every report contains a top-level `$schema` URL pinned to the exact Git commit recorded in `build.commit`, for example `https://raw.githubusercontent.com/trungdlp/ai-gateway-testkit/<commit>/schemas/report.schema.json`. Reports also carry runtime provenance, target fingerprint, catalog version and digest, scenario revisions, assertion-level statuses/reason codes, profile verdicts, and bounded evidence.

Clean `make build` and release builds embed the full source commit automatically. Direct clean Go builds recover the revision from Go VCS build information. Dirty development builds deliberately use `build.commit: unknown` and the mutable `main` schema URL instead of claiming an incorrect immutable revision.

Reports include logical raw HTTP requests, total attempts, recovered retries, and exhausted retry counts. Exhausted transient failures produce `error` and an `INDETERMINATE` profile—not a false incompatibility claim.

`agtk report sanitize` removes endpoint URLs, model IDs, evidence, expected/observed values, and diagnostic messages before sharing. Always use synthetic prompts and test credentials; sanitization is defense in depth, not a data-loss-prevention boundary.

Remote endpoints require HTTPS by default. Loopback HTTP is supported for local fixtures, response bodies are bounded, and OpenAI requests set `store: false`.

See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## Development

```sh
go generate ./...
make check
```

`make check` verifies formatting, the generated catalog, static analysis, race-enabled tests, and the CLI build. Cobra provides the command surface, Testify is used for tests, `slog` emits diagnostics, and the official `openai-go` and `anthropic-sdk-go` clients power SDK interoperability cases.

Read [docs/architecture.md](docs/architecture.md) before adding scenarios or changing report semantics. Contribution expectations are in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).
