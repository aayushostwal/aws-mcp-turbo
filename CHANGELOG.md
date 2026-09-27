# Changelog

## 0.1.2 — Unreleased

- Fixed npm OIDC publishing by avoiding the empty token-auth configuration created by
  `actions/setup-node` when `registry-url` is set without `NODE_AUTH_TOKEN`.
- Updated npx and MCP examples to follow npm's `latest` dist-tag.

## 0.1.1 — 2026-09-27 (GitHub binaries only)

- Expanded the typed read allowlist to 30 AWS services and 61 total operations, including
  STS identity, IAM users, CloudWatch alarms/metrics, RDS instances, and log group discovery.
- Automated binary-first GitHub and npm publication on stable version tags, using npm OIDC.
- Main ancestry/version validation, serialized releases, and immutable npm-version guards.
- Fresh-cache public installation/MCP smoke checks and bounded registry propagation retries.
- Documented trusted-publisher setup and recovery without overwriting published versions.
- npm publication failed; use `0.1.2` or later for npm/npx installation.

## 0.1.0 — 2026-09-27

- Native Go stdio MCP server with exactly four compact tools.
- Twelve read actions across EC2, S3, Lambda, ECS, and CloudWatch Logs.
- Built-in and custom JMESPath projection, recursive pruning, Markdown/TSV/compact JSON.
- Visible pagination cursors and configurable output/deadline limits.
- Bounded session-local CloudWatch forward-cursor polling and cursor checkpoint rings.
- ECS stopped-task and Lambda timeout evidence macros.
- Three opt-in mutation actions with local preview, private durable audit, and approval hook.
- Offline protocol/SDK/concurrency/safety tests, tokenizer reports, and benchmarks.
- CI, vulnerability scanning, draft binary-release workflow, npm launcher, and Homebrew support.
- Public npm launcher under `@ostwal/aws-mcp-turbo`, with symlink startup regression coverage.

This is an initial release. Live AWS and GUI-client acceptance checks, average 85% token savings,
and 3–5× remote-proxy speedup remain unverified or unmet. Windows and macOS notarization are not provided.
