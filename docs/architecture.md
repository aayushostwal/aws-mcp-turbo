# Architecture

The official MCP Go SDK owns stdio JSON-RPC, lifecycle and cancellation. Four small tool
definitions dispatch into an engine with injected action handlers. This keeps protocol
integration independent from AWS calls and allows offline tests at both levels.

```text
cmd/aws-mcp-turbo       Flags, credential configuration, lifecycle, audit/hook wiring
internal/turbo/server   Compact schemas, dispatch, MCP text/error boundary
internal/turbo/registry Explicit allowlist, strict typed inputs, discovery
internal/turbo/aws      AWS SDK v2 service clients and compiled JMESPath presets
internal/turbo/aws_extended  Additional reviewed read-only service bindings
internal/turbo/engine   Read flow, projection, pagination, delta commit
internal/turbo/format   Recursive pruning and deterministic Markdown/TSV/JSON
internal/turbo/cache    Bounded session/query cursor rings, TTL/LRU, striped locks
internal/turbo/diagnose Composite evidence workflows
internal/turbo/mutate   Preview, audit, approval hook, execution and outcome records
```

## Read path

Validate action and read classification → compile override → normalize typed params → acquire
stream lock if needed → call SDK → normalize SDK output to JSON → project → prune → serialize →
append pagination metadata → enforce output bound → commit log cursor → return one text block.

Clients and compiled default projections are created once per process. SDK clients use normal
connection reuse, signing, retries, and credential refresh. Requests have an overall deadline.
No shell, AWS CLI subprocess, generic HTTP dispatcher, or dynamic plugin executes AWS operations.

Each action declares pagination fields separately from its projection. Discovery returns accepted
Go SDK parameter names/types on demand. Input conversion uses typed SDK inputs and rejects unknown
top-level names; nested values are decoded with unknown-field checking. Required-field checks
are local conveniences, not a replacement for SDK/service validation.

## Cache decisions

Forward tokens are more precise than timestamps: multiple valid events share timestamps and
AWS can return partial/empty pages. The bounded ring stores cursor checkpoints and the greatest
observed timestamp, not log bodies. Limits prevent unbounded session retention; a new session
cannot choose another session's cache ID. Fixed lock stripes prevent unbounded lock allocation.
The key includes canonical query parameters and the effective query region. Credentials/account
remain configured for the process; `aws_query` may override the region per call.

Cache commits occur after successful rendering, so output errors can be retried without losing
the page. There is no client acknowledgement transaction, persistent cache, cross-process cache,
or exactly-once promise. See the precise tradeoffs in [the tool reference](tools.md).

## Mutations

Reads and writes are separate allowlist classifications. Writes are never routed through
`aws_query`. The guard requires configuration opt-in, caller intent, durable pre-execution
audit, and an external approval decision for the exact request. The hook is deployment-specific;
the core server provides its contract and fail-closed execution, not a desktop approval UI.

## Scope and extension points

The current registry intentionally supports 30 service clients and 61 operations.
Add actions through typed `bind` calls with projections and explicit read/write classification.
Avoid broad reflection that makes every SDK method callable. Add meaningful adapter/projection,
pagination, and authorization tests for each extension.

Possible follow-ups include more services, richer nested schema discovery, configurable presets,
additional macros, durable opt-in polling checkpoints, and built-in desktop approval providers.
These are not implemented or silently emulated. Benchmark improvements must retain useful
evidence and include the actual tool-output envelope.
