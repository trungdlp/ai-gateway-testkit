# Architecture

AI Gateway Testkit is a black-box conformance and regression system for OpenAI- and Anthropic-compatible gateways. It turns observable API contracts into versioned scenarios, groups their assertions into named compatibility profiles, and emits a canonical report that can be compared or shared without relying on prose interpretation.

The core design separates four questions:

1. Did the runner obtain usable evidence?
2. Which atomic assertions passed or failed?
3. Which scenarios completed, failed, or were blocked by prerequisites?
4. Does the evidence establish a named compatibility claim such as `oai-tools` or `codex-ready`?

## Execution model

```text
target manifest + credentials       embedded profiles
              \                         /
               v                       v
             target --> catalog --> selection
                                   |
                                   v
                         dependency-aware engine
                          /         |          \
                  raw HTTP      official SDK   sbx agent
                          \         |          /
                                   v
                         atomic assertion results
                                   |
                            profile evaluator
                                   |
                      canonical JSON / text report
                                   |
                         compare / sanitize / CI
```

The engine runs scenarios sequentially. This gives deterministic ordering, predictable rate impact, and a bounded request budget. Independent failures do not stop the run. A scenario is blocked only when one of its declared scenario dependencies did not pass.

## Transient failure handling

Raw HTTP and official SDK requests use the run's bounded retry budget. The default is two retries after the initial attempt with deterministic exponential backoff from 250 ms and a maximum wait of five seconds. HTTP `408`, `425`, `429`, `500`, `502`, `503`, and `504`, plus retryable network errors, are eligible. `Retry-After` seconds or HTTP dates are honored but capped by the configured maximum wait.

Authentication failures, other stable 4xx responses, schema mismatches, and assertion failures are never retried. Request JSON is encoded once and replayed byte-for-byte. This is safe from remote mutation for the current catalog, but inference attempts can still be billed more than once; cost-sensitive runs can set `--retries 0`.

When a later attempt succeeds, the assertion is evaluated normally and scenario evidence records the recovered retry. When the budget is exhausted, raw HTTP assertions become `error` with `HTTP.TRANSIENT_EXHAUSTED`; SDK assertions use `SDK.TRANSIENT_EXHAUSTED`. The containing profile is therefore `INDETERMINATE`, because transient unavailability is not evidence of incompatibility. Canonical runtime provenance records retry policy, logical raw HTTP requests, total attempts, recovered retries, and exhausted requests. SDK clients use the same maximum retry count, while their internal attempt totals are not exposed by the upstream libraries and are not included in the raw HTTP counters.

## Repository boundaries

| Path | Responsibility |
| --- | --- |
| `cmd/agtk` | CLI process entry point and build metadata. |
| `cmd/cataloggen` | Reproducibly generates the public catalog artifacts. |
| `cases/*` | Executable scenario definitions grouped by implementation family; each scenario has one ID-named Go file. |
| `internal/testcase` | Public-in-practice vocabulary: definition, assertion, status, cost, risk, environment, and runner contracts. |
| `internal/catalog` | Catalog validation, dependency graph validation, deterministic ordering, version, and digest. |
| `internal/profile` | Embedded, versioned profile definitions and recursive expansion. |
| `internal/target` | YAML target manifest parsing, credential resolution, endpoint validation, and secret-free target fingerprinting. |
| `internal/gateway` | Protocol-neutral bounded HTTP/JSON transport. |
| `internal/engine` | Selection, dependency scheduling, execution normalization, redaction, provenance, and summary counts. |
| `internal/evaluator` | Readiness verdict and coverage/success ratios for each selected profile. |
| `internal/agentrunner` | Optional operational adapter for ephemeral Docker Sandboxes. |
| `internal/result` | Canonical report model. |
| `internal/report` | Text/JSON rendering, strict decoding, and sanitization. |
| `internal/compare` | Assertion-level baseline classification. |
| `catalog/cases/` | One generated machine-readable JSON definition per scenario. |
| `docs/cases/` | One generated human-readable contract per scenario. |
| `schemas/` | JSON Schemas for targets and reports. |

Packages under `internal` may evolve without becoming a Go library compatibility promise. Scenario IDs, assertion IDs, profile IDs and versions, catalog versions, CLI exit codes, and the report schema are user-facing contracts.

## Scenario identity

A scenario ID has the form `<SUITE>-<AREA>-<NNN>` and matches `^[A-Z]{2,4}-[A-Z]{3,5}-[0-9]{3}$`.

Examples:

- `OAI-RESP-001`: OpenAI Responses protocol scenario.
- `ANT-TOOL-001`: Anthropic tool-use protocol scenario.
- `BEH-OAI-001`: model-behavior diagnostic through the OpenAI surface.
- `CDX-EXEC-001`: Codex operational workflow.
- `CLC-EXEC-001`: Claude Code operational workflow.

The numeric suffix is an opaque sequence, not a priority or execution order. IDs are never recycled. Renaming an ID creates a new identity.

Every scenario also has an integer `revision`. Increase the revision when the observable contract, request fixture, or assertion meaning changes. Editorial clarification that cannot alter outcomes does not require a revision. Reports record both ID and revision so results remain interpretable after the catalog evolves.

## Atomic assertions

A scenario contains one or more assertions named `A01`, `A02`, and so on. The globally addressable identity is `<SCENARIO_ID>/<ASSERTION_ID>`, for example `OAI-RESP-001/A04`.

Assertions are atomic because “the response endpoint failed” is not sufficiently actionable. One request may independently establish HTTP success, valid JSON, a discriminator, completion state, model attribution, and non-empty output. Each assertion declares:

- requirement: `must`, `should`, or `may`;
- impact: `blocker`, `high`, `medium`, or `low`;
- title describing one observable condition.

Assertion IDs remain stable within a scenario revision. A changed meaning requires a scenario revision; removing or replacing an externally referenced assertion should generally produce a new scenario ID.

## Status and reason semantics

Assertion statuses are deliberately not boolean:

| Status | Meaning |
| --- | --- |
| `pass` | Evidence established the assertion. |
| `fail` | Evidence established incompatibility. |
| `error` | The runner could not obtain or interpret the required evidence. |
| `blocked` | A prerequisite within the scenario or dependency graph failed. |
| `skipped` | Execution was intentionally disabled, such as an optional agent runner. |
| `not_applicable` | The target does not expose the relevant protocol. |

Stable, namespaced reason codes such as `HTTP.STATUS_UNEXPECTED`, `JSON.INVALID`, `TOOL.CALL_ID_EMPTY`, and `AGENT.RUNNER_DISABLED` support automation without parsing human messages. Messages and evidence are diagnostic and may be removed by sanitization.

A scenario status is the highest-severity status among its assertions. The engine guarantees that every declared assertion appears exactly once in a report; a missing runner result becomes `error` with `RUNNER.ASSERTION_MISSING`.

## Catalog and dependency graph

The catalog is compiled from Go scenario definitions. Startup validation rejects:

- malformed or duplicate scenario and assertion IDs;
- incomplete classification metadata;
- scenarios without a runner;
- references to unknown dependencies;
- dependency cycles.

Catalog definitions are sorted before hashing. The report records the catalog version and SHA-256 digest. Comparison is fully comparable only when digests match; otherwise assertion states are marked `not_comparable` to avoid false regression claims.

Each executable scenario is the source of truth for two generated artifacts: `catalog/cases/<ID>.json` and `docs/cases/<ID>.md`. Keeping artifacts per ID lets contributors add unrelated scenarios without editing a shared generated index. `agtk catalog export` remains the canonical way to produce one aggregate catalog for downstream tools.

Suite packages expose a small registry, and each case file registers its own definition during package initialization. Registration order is irrelevant because catalog construction sorts and validates definitions. The generator also requires every registered ID to map to exactly one `cases/*/<lowercase_id>.go` source file, rejects stale or missing artifacts in check mode, and removes orphan generated artifacts during generation. `go run ./cmd/cataloggen --check` is the drift and layout check used by CI.

## Profiles and compatibility claims

Profiles are versioned YAML definitions embedded in the binary. A profile can include other profiles and can require or optionally reference a whole scenario or one assertion.

For required assertions:

- verdict `PASS`: every required assertion passed;
- verdict `FAIL`: at least one required assertion failed;
- verdict `INDETERMINATE`: no required assertion failed, but at least one is missing, errored, blocked, skipped, or not applicable.

`success_ratio = passed / evaluated`, where evaluated means pass or fail. `coverage_ratio = evaluated / required`. Keeping both prevents unavailable evidence from looking like failure while preventing low coverage from looking like success.

Protocol, SDK, behavioral, and agent claims remain distinct. In particular, a model ignoring an exact-output prompt does not mean the wire protocol is incompatible, and a correct API response does not prove a coding agent can use tools successfully.

Current profile families are listed by `agtk profiles`. Agent readiness profiles are experimental because agent CLIs and sandbox integration evolve more quickly than protocol contracts.

## Target model

A target manifest may configure OpenAI, Anthropic, or both with independent base URLs, models, credentials, and Anthropic API versions. Credentials are referenced by environment-variable name and resolved only at runtime.

```yaml
name: staging
openai:
  base_url: https://gateway.example.com/v1
  model: model-for-responses
  credential_env: OPENAI_GATEWAY_API_KEY
anthropic:
  base_url: https://gateway.example.com/v1
  model: model-for-messages
  credential_env: ANTHROPIC_GATEWAY_API_KEY
```

The target fingerprint hashes public endpoint configuration, never credential values. Legacy single-endpoint flags remain a convenience adapter and produce the same resolved target model.

## Evidence, provenance, and redaction

The canonical report records schema/catalog versions, catalog digest, run ID, start time, duration, build metadata, Go runtime, SDK versions, target fingerprint, selected profiles, verdicts, scenario revisions, assertion results, and bounded evidence previews.

Schema version 1.1 adds a top-level `$schema` URL. For clean builds it points to the raw schema file under the exact 40-character Git commit recorded in `build.commit`, making validation independent of future changes on `main`. The decoder verifies that these values agree. `make build` injects the clean source commit, while direct Go builds recover clean VCS metadata from the runtime build information. A dirty development build cannot truthfully reference an unpublished commit, so it records `unknown` and uses the `main` schema URL as an explicit mutable fallback.

The checked-in report and target schemas use canonical `$id` values on `raw.githubusercontent.com`, where validators can retrieve JSON directly. GitHub repository paths without `/blob/<ref>/` are not file URLs, while `/blob/` URLs return HTML and therefore are not suitable schema identifiers.

Credentials are injected only into intended authorization mechanisms. The live environment redacts all configured credential values and the synthetic invalid credential from error messages and evidence. Transport bodies are bounded separately. `agtk report sanitize` additionally removes endpoint URLs, model IDs, evidence, expected/observed values, and diagnostic messages for safer sharing. The target name and fingerprint remain so reports from the same public configuration can still be correlated.

Sanitization is a sharing aid, not a substitute for using synthetic prompts and non-production data.

## Agent execution with Docker Sandboxes

Agent scenarios run only when `--agent-runner sbx` is selected. The runner:

1. creates a private host temporary directory and deterministic task fixture;
2. scopes a Docker custom secret to the sandbox and gateway host so the agent receives only a proxy placeholder;
3. creates a uniquely named Codex or Claude sandbox and a gateway-host-only network allow rule;
4. invokes the agent non-interactively with session persistence disabled;
5. verifies the requested file content and a marker produced only by the fixture validator;
6. removes the named sandbox and both temporary locations.

The separate mode-`0600` env file contains only the base URL and model and lives outside the mounted workspace. Codex uses `exec --ephemeral`; Claude Code uses print mode with session persistence disabled. Permission bypass is confined to the outer Docker sandbox and disposable fixture. The host repository is never mounted for these cases. If `sbx` execution is not enabled, agent assertions are `skipped`, making an agent-readiness profile `INDETERMINATE` rather than producing a false pass.

## Baseline comparison

Comparison operates on global assertion IDs and classifies each as:

- `new_failure`;
- `resolved`;
- `unchanged_failure`;
- `unchanged_pass`;
- `not_comparable`.

Any non-pass status is failure-like for change classification, while the original detailed status remains available. `agtk run --baseline` annotates the current canonical report. `agtk compare` produces a focused two-report comparison.

## Adding a scenario

Before implementing a roadmap item, review the accepted [test case roadmap proposal](proposals/test-case-roadmap.md). A reserved proposal ID is not an executable compatibility contract until its source definition, deterministic tests, generated artifacts, and intended profile change are merged.

1. Choose the correct layer and a permanent suite/area ID.
2. Create exactly one source file named from the lowercase ID with hyphens replaced by underscores, for example `cases/openai/oai_resp_002.go`.
3. State observable assertions before writing request code, build the complete definition in that file, and call the package's `register` function from `init`. Do not edit a shared case list.
4. Declare stability, determinism, spec references, dependencies, bounded cost, and risk.
5. Implement a runner with synthetic fixtures and no remote mutation unless explicitly classified and cleaned up.
6. Add local `httptest` coverage for successful, incompatible, and unavailable-evidence paths.
7. Add the scenario or assertion to the narrowest appropriate profile only when the compatibility claim should change. Profile files are intentionally shared semantic contracts, so concurrent edits may require reconciliation.
8. Run `go generate ./...`. Commit the new `catalog/cases/<ID>.json` and `docs/cases/<ID>.md` alongside the source file.
9. Run `make check`.

Do not weaken an assertion so more providers pass. Represent an optional capability with a separate profile or optional reference. Do not mix model-quality diagnostics into protocol claims. A scenario should be split when its requests, dependencies, lifecycle, or failure ownership differ materially.

## Versioning policy

- Report schema: semantic version; breaking field or meaning changes increment the major version.
- Catalog: date-oriented version plus digest; any executable definition change updates the digest, and releases update the catalog version.
- Profile: semantic version per compatibility claim.
- Scenario: stable ID plus monotonic integer revision.
- Reason code: stable machine contract; introduce a new code when failure meaning changes.

This layered versioning lets two teams say “`oai-tools` 1.0.0 failed at `OAI-TOOL-001/A04` under catalog digest X” and mean exactly the same thing.
