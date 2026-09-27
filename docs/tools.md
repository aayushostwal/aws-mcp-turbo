# Tool reference

All results are a single MCP text content block. Errors set `isError:true`.
Raw SDK responses are never duplicated in `structuredContent`. Default result limit is 32 KiB;
requests larger than 64 KiB are rejected by the handler. Transport parsing happens before
that handler limit, so this is intended for a trusted local client, not a public server.

## Discovery

`aws_discover({"search":""})` returns the compact action/preset catalog and macro descriptions.
Search is a case-insensitive substring. An **exact, case-sensitive action name**, for example
`lambda.GetFunctionConfiguration`, also returns accepted SDK parameter fields and Go types.
Nested parameter structure follows the corresponding AWS SDK Go input type.
Required fields are listed separately; the AWS SDK/service performs additional validation.

## Queries and projections

Arguments: `action` (required), `params` (object, default `{}`), `projection` (JMESPath override),
and `format` (`markdown`, `tsv`, or `json`; default Markdown for lists, compact JSON otherwise).

Action names are `service.Operation` and case-sensitive. Parameters use **Go SDK field names**:
`FunctionName`, `LogGroupName`, `InstanceIds`, `Filters`, etc. They are not CLI kebab-case names
or CloudWatch wire JSON names. Unknown fields, wrong types, and missing required fields fail.
Only registered read actions are accepted, even if an unregistered action starts with `Get`.

JMESPath operates on JSON-normalized SDK output, retaining PascalCase field names. Presets are
compiled once. Overrides compile before contacting AWS and are still pruned and size-limited.
`projection:"@"` is allowed for a small full response but still removes SDK metadata and empty
containers. A projection is data selection, not a security/redaction boundary.

Pruning removes `null`, `[]`, and `{}` recursively. It keeps `false`, numeric zero, and `""`.
Completely empty results render as `No results.` (including `format:"json"`). Tables have a
sorted union of columns; nested values use compact JSON. TSV and Markdown cells escape
tabs/newlines/backslashes; Markdown also escapes pipes. The format is for model consumption,
not spreadsheet import. Object roots stay compact JSON in every format.

## Pagination

Each query issues one API call (plus SDK retries). A returned continuation cursor is appended
as `next_params={...}`, outside the projection, so it cannot be accidentally projected away.
Merge those fields into the same input parameters and call again. `format:"json"` selects the
body format; the complete text result may include the pagination trailer and is not always
a single JSON document. Empty pages may still contain a continuation cursor.

| Actions | Output token → next input parameter |
| --- | --- |
| EC2 describes, `ecs.ListTasks`, `logs.FilterLogEvents` | `NextToken` → `NextToken` |
| `s3.ListBuckets` | `ContinuationToken` → `ContinuationToken` |
| `s3.ListObjectsV2` | `NextContinuationToken` → `ContinuationToken` |
| `lambda.ListFunctions` | `NextMarker` → `Marker` |
| `logs.GetLogEvents` with explicit token | `NextForwardToken` → `NextToken` |
| IAM lists, RDS describes, ElastiCache describes | `Marker` → `Marker` |
| ELBv2 describes, `route53.ListHostedZones`, `kms.ListKeys` | `NextMarker` → `Marker` |
| `dynamodb.ListTables` | `LastEvaluatedTableName` → `ExclusiveStartTableName` |
| `apigateway.GetRestApis` | `Position` → `Position` |
| Most other new list/describe actions | `NextToken` → `NextToken` (when present) |

Use `aws_discover` to see the exact case-sensitive action names and accepted Go SDK input
fields. For example, `cloudwatch.DescribeAlarms` is supported; `monitoring.DescribeAlarms`
is not an alias. The new `cloudwatch.GetMetricStatistics` binding requires `Namespace`,
`MetricName`, `StartTime`, `EndTime`, and `Period`; AWS also expects `Statistics` or
`ExtendedStatistics`. `secretsmanager.DescribeSecret` returns metadata, not secret values.

The server does not silently truncate rows or fetch an unbounded number of pages.
An oversized response fails with guidance to reduce `MaxResults`, `MaxKeys`, `Limit`, filters,
or projection. The output byte ceiling is a bound, not a fixed token budget.

## Delta log polling

Without an explicit `NextToken`, `logs.GetLogEvents` maintains an in-memory cursor keyed by
the actual MCP session and canonical query parameters. Default `Limit` is 200 and default
`StartFromHead` is true. Choose `StartTime` to avoid reading the full history. Subsequent
identical calls advance using the AWS forward token with `StartFromHead:true`.

Forward cursors preserve distinct equal-timestamp events and AWS pagination ordering.
Timestamp-only filtering would lose valid events, so timestamps are recorded for checkpoints
but never used to discard rows. There is no content-based deduplication. Delivery follows
AWS's cursor behavior and is not an exactly-once guarantee.

- Cache: at most 256 session/query streams; 16 cursor checkpoints per stream.
- Inactive streams expire after 30 minutes; least recently used entries are evicted at capacity.
- Tokens are discarded after 23 hours to stay below AWS's 24-hour validity window.
- Restart, eviction, or expiration replays from the original input range. No durable state.
- Same-stream concurrent calls serialize; other streams can progress independently, except
  occasional shared lock-stripe contention.
- API, projection, or formatting failures do not advance the cursor. Successful formatting
  advances it before transport delivery; a disconnected client can miss an already-committed page.
- Changing projection/format does not reset the cursor. Changing parameters creates a new stream.
- Supplying `NextToken` bypasses the cache; set `StartFromHead:true` for a forward token.
- The cache cannot be reset by another session. To reread, change `StartTime` or start a new session.

See [AWS GetLogEvents semantics](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_GetLogEvents.html).

## Diagnostic macros

`aws_diagnose` accepts `macro` and `params`. Macros gather evidence under one overall deadline.
They are deterministic workflows; they do not assert a definitive root cause from incomplete data.

### ecs_task_stopped

`Cluster` is required; optional `Tasks` is an array of up to 100 task ARNs/IDs. Without `Tasks`,
the macro lists one page of stopped tasks, then describes them. Output retains AWS stop reasons,
container exit codes/reasons, and `Failures`. Additional list pages are explicitly identified
with continuation arguments. An empty first page does not prove there are no stopped tasks.

### lambda_timeout

`FunctionName` is required; optional `LogGroupName` overrides `/aws/lambda/<function-name>`.
`StartTime` is an optional epoch-millisecond timestamp (default: one hour ago). Function ARNs
and qualifiers are handled when deriving the default log group.

Configuration and one page of up to 100 matching log events are fetched concurrently. The filter
matches both `Task timed out` and `Status: timeout`. Functions using custom log groups must supply
`LogGroupName`. Custom JSON log structures and future runtime messages may require a separate
query. Partial permission/API failures are labeled `Unavailable`; both sources failing returns
a tool error. No matches is not proof that the function is healthy.

## Mutations

Arguments: `action`, `params`, nonblank `intent`, and optional `execute` (default false).
All calls are rejected unless enabled in server configuration. Enabled previews validate local
input types/required fields and audit the request, without calling AWS. Execution additionally
requires approval from the configured hook. Only the three registered write actions are accepted.
See [SECURITY.md](../SECURITY.md) for the complete contract and operational caveats.
