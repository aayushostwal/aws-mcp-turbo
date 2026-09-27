# Changelog

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
