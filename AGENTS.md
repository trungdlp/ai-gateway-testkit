# AGENTS.md

This file guides coding agents working in AI Gateway Testkit. Keep changes small, preserve public compatibility contracts, and verify behavior before handing work off.

## Project map

- `cmd/agtk`: Cobra CLI entry point and build metadata.
- `cmd/cataloggen`: deterministic per-case JSON and Markdown generator.
- `cases/<family>`: executable test cases and suite-local registries.
- `internal/catalog`: case validation, ordering, dependency graph, version, and digest.
- `internal/profile`: embedded compatibility-profile definitions.
- `internal/engine`: selection, execution, retry evidence, redaction, and summaries.
- `internal/result`, `internal/report`, `schemas`: canonical report contract.
- `catalog/cases` and `docs/cases`: generated artifacts; never edit them manually.

Read `docs/architecture.md` before changing scenario, assertion, profile, catalog, report, or verdict semantics.

## Test-case contribution contract

Each test case has one executable source file. Convert its stable ID to lowercase and replace hyphens with underscores: `OAI-RESP-001` belongs in `cases/openai/oai_resp_001.go`.

Build the entire case definition in that file and register it from `init()` with the package-local `register` function. Do not create or edit a central list when adding a case. Shared `base.go` helpers contain only family-wide defaults; `registry.go` contains only registration mechanics.

IDs are permanent public contracts and are never recycled. Increment `Revision` when a request fixture, observable contract, or assertion meaning changes. Assertion IDs are globally referenced as `<CASE-ID>/<ASSERTION-ID>`. Keep assertions atomic and use stable reason codes; distinguish incompatibility (`fail`) from unavailable evidence (`error`, `blocked`, `skipped`, or `not_applicable`).

Declare dependencies, stability, determinism, specification references, bounded request/token cost, mutation risk, cleanup, and model-output exposure. Use synthetic fixtures. A case that differs materially in request lifecycle, dependencies, or failure ownership should have a separate ID.

Profiles are compatibility claims, not discovery lists. Change a profile only when the new or revised assertion is intentionally part of that named claim. Shared profile edits may conflict and require semantic review.

After changing cases or profiles, run `go generate ./...`. Commit the generated `catalog/cases/<ID>.json` and `docs/cases/<ID>.md` files. Generation removes orphan case artifacts; CI rejects missing, stale, orphaned, or ambiguously sourced artifacts.

## Implementation rules

- Prefer the smallest change that satisfies the requested observable behavior.
- Match existing Go style and package boundaries. Do not refactor unrelated code.
- Use Cobra for CLI commands, Testify for test assertions, and `log/slog` for diagnostics.
- Use the official `openai-go` and `anthropic-sdk-go` clients for SDK interoperability cases.
- Exercise gateway behavior with local `httptest` servers. Unit tests must be deterministic, offline, and free of API charges.
- Keep retries bounded and limited to transient transport failures and documented transient HTTP statuses. Stable client, authentication, and schema failures must not be retried.
- Preserve bounded response bodies, credential redaction, HTTPS validation, and secret-free reports. Never commit credentials, tokens, production prompts, or live response bodies.
- Treat report schema fields, scenario/assertion/profile IDs, versions, reason codes, and CLI exit codes as public interfaces.

## Verification

Use targeted tests while developing, then run the full gate:

```sh
go test ./path/to/changed/package
go generate ./...
make check
```

Before finishing, inspect `git diff`, confirm generated artifacts match their source cases, and scan the diff for secrets. Use conventional commit messages. Follow `.github/pull_request_template.md` exactly when opening a pull request.
