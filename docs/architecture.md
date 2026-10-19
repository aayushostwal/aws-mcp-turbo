# Architecture

The official MCP Go SDK owns stdio JSON-RPC, lifecycle and cancellation. Three small tool
definitions dispatch into an engine. The engine finds an action by name, runs it, and shapes the
answer. Actions come from the AWS SDK itself, so the engine does not list operations by hand.

```text
cmd/aws-mcp-turbo       Flags, credential configuration, lifecycle, audit and hook wiring
internal/turbo/server   Compact schemas, dispatch, MCP text/error boundary
internal/turbo/engine   Query flow: fields, pagination, batching, regions, cache, discovery
internal/turbo/dynamic  Service set: SDK operations by reflection, read/write rules, defaults, guards
internal/turbo/services Table of the 76 bundled AWS service clients
internal/turbo/presets  Hand-tuned views for common reads
internal/turbo/recipes  Investigations as data (parallel reads with placeholders)
internal/turbo/diagnose Recipe dispatch plus the two recipes that need a first step (ECS, Lambda)
internal/turbo/typedjson DynamoDB-style typed inputs the standard JSON decoder cannot build
internal/turbo/format   Pruning, compact/markdown/tsv/json rendering, truncation to fit
internal/turbo/readcache Short-lived answer cache
internal/turbo/cache    Bounded session/query cursor rings for log polling
internal/turbo/mutate   Preview, audit, approval hook, execution and outcome records
```

## Where actions come from

Every AWS SDK service client has methods shaped `Op(ctx, *OpInput, ...opts) (*OpOutput, error)`.
`services.go` lists the clients. On first use of a service the server reads its method set with
reflection, which needs only the Go type, not a client, and keeps a sorted index. The first call
to an action then builds it once:

- **Read or write** is decided from the operation's verb (see [SECURITY.md](../SECURITY.md)).
- **Parameters** are the input struct's exported fields. Names are matched exactly, so cache keys,
  approval records and the SDK always see the same field names.
- **Pagination** is found by matching response tokens to request fields: `NextToken`, `Marker`,
  `NextMarker` → `Marker`, `NextContinuationToken` → `ContinuationToken`, `LastEvaluatedKey` →
  `ExclusiveStartKey`, and so on.
- **The default view** comes from a preset if one exists. Otherwise it is derived from the output
  type: the main list or record is kept; each record shows its own identifiers (`SubnetId` on a
  `Subnet`), then up to three state or status fields, then up to two timestamps or endpoints, capped at
  eight fields. Paging and bookkeeping fields are dropped. Wrapper records that only carry a list
  (CloudFront's `DistributionList.Items`) are looked through.
- **The call** decodes the request into a fresh input struct, invokes the method by reflection, and
  converts the output to plain JSON. Union values such as DynamoDB attributes are unwrapped.

Clients are created per service and region on first use and reused. The region travels in the
request context, so a per-call region needs no second registry. Startup cost is the process start,
about 40 ms; the 76 services are indexed only when searched or called.

Reflection is deliberate here. The alternative was one hand-written binding per operation, which
is why the earlier release stopped at 61. The safety rules moved from a list of operations to rules
over names (reads), a deny list (credentials), flag guards (decryption) and the type system (no
streams), all tested across every operation in `dynamic_test.go`.

## Read path

Validate action, region and format → resolve fields or projection → check the 15-second cache →
for log polling, take the stream lock and resume from the cursor → validate and call the SDK
→ project → if `all`, follow the token (up to 25 pages or twice the output limit) → group if asked
→ prune → render compact rows within the byte limit → append `next_params`, `pages` or
`truncated` notes → commit the log cursor → return one text block.

`queries`, `regions` and recipes are fan-outs built on one bounded worker pool (four at a time).
Each branch gets an equal share of the output limit, so one large answer cannot crowd out the rest.
Results keep request order, and a failed branch is reported next to the successful ones.

## Output shaping

Most savings come from how rows are written, not from fetching less:

- Tab-separated rows with no table decoration.
- Values that every row shares are printed once (`all:`), shared ARN or path prefixes are stated
  once (`prefix:`), seconds are dropped from UTC timestamps, and empty values are removed.
- Rows that do not fit the limit are dropped from the end, and the result says so. A page that is
  cut can be re-requested with `fields` or tighter parameters.
- Single records print as `Key=value` lines.

## Cache decisions

Log polling uses forward tokens rather than timestamps: events share timestamps and AWS can return
partial or empty pages. The bounded ring stores cursor checkpoints and the greatest observed
timestamp, not log bodies. A new session cannot choose another session's cache ID. Fixed lock stripes
bound lock allocation. The key includes the canonical parameters and the effective region.
Cursors commit only after rendering succeeds, so an output error can be retried without losing the page.
There is no persistent or cross-process cache and no exactly-once promise.

The read cache is separate: it stores finished text for 15 seconds, is cleared by any executed
mutation, and never holds log polling.

## Mutations

Reads and writes are separate classes and writes never go through `aws_query`. The guard requires
configuration opt-in, caller intent, a durable pre-execution audit record, and an external approval
of the exact request. The hook is deployment-specific; the core provides its contract and fails closed.

## Scope and trade-offs

- Binary size is the cost of bundling 76 services (about 190 MB, about 40 MB compressed). Dropping
  a service is deleting its line in `services.go`.
- Default views are heuristics. They are tested on representative operations, and `fields`,
  `projection` and `aws_discover` (which lists what an action returns) cover the rest.
- Operations that only make sense with byte streams are absent.
- Possible follow-ups: presigned URLs for object content, CloudWatch Logs Insights as a recipe,
  per-service presets as real usage shows where the defaults fall short.
