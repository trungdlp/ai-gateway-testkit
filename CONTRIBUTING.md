# Contributing

Thanks for helping make AI gateway compatibility measurable and reproducible.

## Before opening a change

- Search existing issues and pull requests.
- For a new protocol surface or test category, open an issue describing the observable contract and a minimal reproduction.
- Never include live API keys, customer prompts, production response bodies, or other sensitive data.

## Local workflow

Use Go 1.25 or newer.

```sh
git clone git@github.com:trungdlp/ai-gateway-testkit.git
cd ai-gateway-testkit
make check
```

Changes should be small and test-driven. Gateway behavior must be tested with `httptest` fixtures so the default test suite remains deterministic, offline, and free of usage charges.

Scenario and assertion IDs are public interfaces. Follow the identity, revision, status, reason-code, and profile rules in [docs/architecture.md](docs/architecture.md). After changing a case or profile, run `go generate ./...` and include the generated catalog changes.

## Pull requests

A pull request should:

- explain the gateway behavior being asserted;
- include tests for success and failure paths;
- preserve credential redaction and bounded I/O;
- update user-facing documentation when flags, checks, output, or exit codes change;
- pass `make check`.

The JSON report is a public interface. Backward-incompatible report changes require a schema version change and migration notes.

By contributing, you agree that your contributions are licensed under the Apache License 2.0.
