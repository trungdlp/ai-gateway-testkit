# Security Policy

## Supported versions

Until the first stable release, security fixes are applied to the latest commit on the default branch.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting for this repository. Do not open a public issue for a suspected vulnerability and do not include real credentials or customer data in a report.

Include the affected commit or version, impact, reproduction steps using synthetic data, and any suggested mitigation. You should receive an acknowledgement within five business days.

## Credential handling

The project treats gateway credentials as secrets. Tests and logs must never print API keys. Fixtures must use obviously synthetic values. If a credential is accidentally committed or posted, revoke it immediately; removing it from Git history is not a substitute for rotation.

Agent tests use a Docker custom-secret placeholder scoped to one ephemeral sandbox and the configured gateway host. The host repository is not mounted, and sandbox-scoped network access and secrets are removed with the sandbox.
