# Tool reference

Every result is a single MCP text block. `aws_discover` also returns the same data as
`structuredContent`. Errors set `isError:true`. Default result limit is 32 KiB; requests larger
than 64 KiB are rejected. The server is meant for a trusted local client, not as a public service.

## aws_discover

`{"search":""}` lists the services. `{"search":"lambda list"}` lists actions whose names contain
every word (case-insensitive), reads first and writes marked `"write":true`, up to 60 at a time with
the total count. A bare service name such as `ec2` lists that service. Add words to narrow.

An **exact, case-sensitive action name** such as `lambda.GetFunctionConfiguration` returns:

- `parameters`: accepted input fields and their Go types (`Filters` is a list of `types.Filter`);
- `returns`: the top-level fields of the response, useful for writing `fields`;
- `default_projection`: what you get without `fields` or `projection`;
- `required`: fields checked locally before the call (AWS validates the rest);
- `write`: whether the action changes state.

It also lists matching recipes under `macros`.

## aws_query

Provide exactly one of `action`, `queries` or `macro`.

| Argument | Meaning |
| --- | --- |
| `action` | `service.Operation`, case-sensitive. Reads only; writes use `aws_mutate`. |
| `params` | Input object using exact Go SDK field names (`FunctionName`, `InstanceIds`, `Filters`). Unknown names, wrong types and missing required fields fail. |
| `fields` | Comma-separated names to keep, e.g. `"InstanceId,State.Name"`. Applies to the response's main list or record. A dotted path is reached through nested records and its key is the last name. |
| `projection` | A JMESPath expression for full control. Not combined with `fields`. |
| `format` | `compact` (default), `markdown`, `tsv` or `json`. |
| `group` | Name of a field in the result; returns how many rows have each value, most common first. Add that field to `fields`. |
| `all` | Follow continuation tokens until the end, the output limit, or 25 pages. The result states `pages=N`. |
| `region` / `regions` | One region for this call, or up to eight, run four at a time. Results are labeled and keep request order. Cannot be combined. |
| `queries` | Up to eight single-action queries run in parallel. Each result is labeled `### 1 service.Operation`. A failed query does not hide the others; the output ends with `partial=true`. Each query gets an equal share of the output limit. |
| `macro` + `params` | A recipe (below). |

### Output format

The default `compact` format is built for models, not spreadsheets:

- A list of records becomes a header row and tab-separated rows.
- Columns where every row (at least three) has the same value are printed once on an `all:` line.
- Columns whose values share a prefix ending in `:` or `/` (typically an ARN) print the prefix once on a
  `prefix:` line; add it back to each value.
- ISO timestamps in UTC lose seconds: `2026-10-11 09:41Z`.
- A single record becomes `Key=value` lines; nested values stay as compact JSON.
- Identifying columns (`Id`, `Name`, `Arn`) come first, `Count` last, the rest alphabetically.
- `null`, `[]` and `{}` are removed, while `false`, `0` and `""` are kept. SDK metadata is removed.
- An empty result prints `No results.`

`markdown` and `tsv` give plain tables; `json` gives compact JSON. A projection selects data. It is not
a redaction boundary.

### Limits and truncation

A result that does not fit the output limit is cut to the rows that fit, with
`truncated: S of T rows; narrow params, use fields, or group`. Only if not even one row fits does the
call fail. Log polling never truncates, because dropped events could not be recovered.

### Pagination

By default one AWS page is returned. A continuation appears as `next_params={...}`: merge it into
`params` and call again, keeping the same region. With `all:true` the server does this for you.
The tokens come from the SDK types, so every paginated operation works, including DynamoDB's
`LastEvaluatedKey`, which is returned in request form so it can be passed back unchanged.

### Caching

A repeated identical read inside 15 seconds is answered from memory (up to 128 entries). Any
executed mutation clears it. Log polling is never cached this way.

### DynamoDB items

Items are returned as plain values (`{"pk":"user#1","age":"41"}`; numbers stay strings, as DynamoDB
sends them). Inputs that hold attribute values use DynamoDB's own JSON:
`"ExpressionAttributeValues":{":p":{"S":"user#1"},":n":{"N":"5"}}`. A bare JSON number is accepted for `N`.
Types: `S N B BOOL NULL L M SS NS BS`.

### Errors

AWS failures show the error code and message, `UnauthorizedOperation: You are not authorized...`,
without SDK request details. Expired credentials say so and suggest `aws sso login`.

## Delta log polling

Without an explicit `NextToken`, `logs.GetLogEvents` keeps an in-memory cursor per MCP session,
effective region and exact parameters. Default `Limit` is 200 and `StartFromHead` is true. Choose
`StartTime` to avoid reading the whole history. Repeating the same call returns only newer events.

Forward tokens preserve distinct events with equal timestamps. There is no content deduplication
and no exactly-once promise.

- At most 256 streams, 16 checkpoints each; inactive streams expire after 30 minutes.
- Tokens are dropped after 23 hours, inside AWS's 24-hour validity window.
- A restart, eviction or expiry replays from the original range. Nothing is stored on disk.
- Failures in the API, projection or formatting do not advance the cursor.
- Supplying `NextToken` bypasses the cursor. A new session starts fresh.

See [AWS GetLogEvents semantics](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_GetLogEvents.html).

## Recipes

Run with `{"macro":"name","params":{...}}`. A recipe runs its reads in parallel under one deadline and
labels each section. They gather evidence; they do not claim a root cause. `aws_discover` lists them.

| Recipe | Parameters | Returns |
| --- | --- | --- |
| `ecs_task_stopped` | `Cluster`; optional `Tasks` (up to 100) | Stop reasons, container exit codes and failures. Without `Tasks`, one page of stopped tasks is listed first and further pages are named. |
| `lambda_timeout` | `FunctionName`; optional `LogGroupName`, `StartTime` (epoch ms) | Configuration and up to 100 recent timeout log lines. The log group is derived from the name or ARN; custom log groups need `LogGroupName`. No matches is not proof of health. |
| `lambda_errors` | `FunctionName` | Configuration and, for the last hour, invocations, errors, throttles and duration. |
| `ec2_instance` | `InstanceId` | Instance details, status checks with scheduled events, CPU for the last hour. |
| `rds_status` | `DBInstanceIdentifier` | Instance state, CPU and connections for the last hour, events for the last day. |
| `ecs_service_status` | `Cluster`, `Service` | Counts, deployment rollout state, latest service events. |
| `alb_target_health` | `TargetGroupArn` | Each target's health with the reason. |
| `cost_by_service` | none | Unblended cost per service, last 30 days (Cost Explorer). |
| `cloudtrail_recent` | optional `Username` | API activity from the last hour. |

Recipes are data in `internal/turbo/recipes.go`: a list of reads with `$Param` placeholders and relative
times (`@now-1h`). Adding one is a table entry and a test.

## aws_mutate

Arguments: `action`, `params`, a nonblank `intent`, optional `region` and optional `execute` (default false).
Calls fail unless the server was started with `--enable-mutations`. A preview validates the input
locally and writes an audit record without calling AWS. Execution also needs the approval hook to approve
the exact request. See [SECURITY.md](../SECURITY.md).

Any SDK operation that is not a read can be previewed or approved this way, except the operations that
return credentials or key material, which are not callable at all (`sts.AssumeRole`, `iam.CreateAccessKey`,
`kms.Decrypt`, `secretsmanager.GetSecretValue`, and similar).
