# Changelog

## 0.2.0 — Unreleased

A rewrite of how actions are found and how answers are written. Behaviour that existed in 0.1.x
keeps working except where listed under "Changed".

### Added

- 76 AWS services and about 2,700 read operations, taken from the AWS SDK clients instead of a hand-written
  list of 61. New services are one line in `internal/turbo/services.go`. Clients are created on first use.
- Any SDK operation that is not a read can be previewed or approved through `aws_mutate`. Operations that
  return credentials or key material are not callable.
- `aws_query` gains `fields`, `group`, `all`, `queries` (up to eight parallel reads) and `macro`.
- Nine recipes: the two existing ones plus `lambda_errors`, `ec2_instance`, `rds_status`,
  `ecs_service_status`, `alb_target_health`, `cost_by_service` and `cloudtrail_recent`.
- Compact output format, now the default (shared values and prefixes stated once, short timestamps).
- A 15-second read cache, cleared by executed mutations.
- DynamoDB items as plain values on output and DynamoDB JSON on input.
- Search by several words in `aws_discover`; an exact action also lists what it returns.
- `scripts/bench-compare.mjs` and `bench/tasks.json` for comparing with another MCP server.

### Changed

- **Three tools.** `aws_diagnose` is now `aws_query` with `macro`.
- A response that is too large is cut to the rows that fit and says so, instead of failing.
- Errors show the AWS error code and message only.
- `aws_mutate` accepts an optional `region`.
- Releases include a gzip copy of each binary (about 40 MB instead of about 190 MB) that the npm launcher uses.
- Reads are recognised by operation name, with a deny list and flag guards, instead of a list of operations.
  See [SECURITY.md](SECURITY.md).

### Fixed

- `scripts/token_metrics.py --help` crashed on an unescaped percent sign.

## 0.1.4 — Unreleased

- Added structured discovery results alongside text for MCP clients that consume structured tool output.
- Added bounded parallel `aws_query.regions` fan-out for the same read action across up to eight regions.

## 0.1.3 — 2026-09-27

- Added per-query `aws_query.region` override while preserving the configured default.
- Kept CloudWatch log polling cursors isolated by region.

## 0.1.2 — 2026-09-27

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
