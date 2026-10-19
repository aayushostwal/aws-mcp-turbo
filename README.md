# aws-mcp-turbo

[![CI](https://github.com/aayushostwal/aws-mcp-turbo/actions/workflows/ci.yml/badge.svg)](https://github.com/aayushostwal/aws-mcp-turbo/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A local AWS MCP server built to spend as few tokens and round trips as possible.
Three small tools reach **76 AWS services and about 2,700 read operations**, answer in a compact
text format, and send writes through a preview, audit and approval gate.
It runs over stdio on your normal AWS credentials. There is no hosted endpoint, proxy or login flow.

**Status:** 0.2. Offline tests cover the protocol, request encoding, projections, pagination,
caching, batching and mutation gates. Live-account and desktop-client runs are still your check:
see [benchmarks](docs/benchmarks.md) for what is measured and what is not.

## Why it exists

An agent usually needs an instance's identity, state and address, not every field of
`DescribeInstances`. The server keeps the model's context small in four ways:

- **Small tool list.** Three tools, about 270 tokens in total. Which actions exist is looked up on demand.
- **Small answers.** Every response is cut to useful fields, empty values are removed, and rows
  are written as plain tab-separated text. Values every row shares are stated once.
- **Few calls.** `all` pages a list to the end, `queries` runs up to eight reads in parallel,
  and `macro` runs a ready-made investigation in one call. No script has to be written per question.
- **No repeats.** The same read inside 15 seconds is answered from memory, and log polling only
  returns events it has not shown yet.

```text
Agent / IDE ── stdio MCP ── aws-mcp-turbo ── AWS SDK v2 ── AWS APIs
                                │
                  fields → prune → compact rows → fit to limit
                  session log cursors / mutation audit + approval
```

## Quick start

The recommended installation is **npx** (Node.js 22+, macOS or Linux, arm64 or amd64).
It downloads a checksum-verified native binary the first time; no Go compiler is needed.

```sh
npx -y @ostwal/aws-mcp-turbo@latest --version
```

Use an existing AWS profile. For SSO, sign in first with `aws sso login`. Then add this to your client:

```json
{
  "mcpServers": {
    "aws-turbo": {
      "command": "npx",
      "args": ["-y", "@ostwal/aws-mcp-turbo@latest", "--region", "us-west-1"]
    }
  }
}
```

See [client configuration](docs/clients.md) and [installation](docs/installation.md) for source
builds, verified binaries and Homebrew.

## Three tools

| Tool | Purpose |
| --- | --- |
| `aws_discover` | Find actions and recipes by words. An exact action name shows its parameters and what it returns. |
| `aws_query` | Read AWS: one action, a parallel batch, several regions, or a recipe. |
| `aws_mutate` | Preview a write. Execution needs server opt-in and an approval hook. |

Examples below are tool arguments, not shell commands.

Find what exists. Several words must all match:

```json
{"search":"lambda list"}
```

Read, keeping only some fields, across every page:

```json
{"action":"iam.ListRoles","fields":"RoleName,CreateDate","all":true}
```

Count instead of listing:

```json
{"action":"ec2.DescribeInstances","fields":"InstanceId,State.Name","group":"State.Name","all":true}
```

Run independent reads in one call:

```json
{"queries":[
  {"action":"sts.GetCallerIdentity"},
  {"action":"ec2.DescribeInstances","regions":["us-west-1","ap-south-1"]},
  {"action":"cloudwatch.DescribeAlarms","params":{"StateValue":"ALARM"}}
]}
```

Run a recipe:

```json
{"macro":"rds_status","params":{"DBInstanceIdentifier":"prod-db"}}
```

Example output:

```text
all: Runtime=python3.13 State=Active
prefix: Arn=arn:aws:lambda:us-west-1:123456789012:function:
Name	Arn	Modified
api	api	2026-10-11 09:41Z
worker	worker	2026-10-12 09:41Z
cron	cron	2026-10-13 09:41Z
```

The [tool reference](docs/tools.md) covers parameter casing, pagination, limits, errors, DynamoDB
items, the output format and every recipe.

## What it can reach

76 services are built in, including EC2, S3, Lambda, ECS, EKS, RDS, DynamoDB, IAM, CloudWatch,
CloudWatch Logs, CloudFormation, Route 53, CloudFront, SQS, SNS, Step Functions, Athena, Glue,
Cost Explorer, Organizations, Security Hub, GuardDuty and Config.
Run `aws_discover` with no search for the full list.

Any operation whose name starts with `List`, `Describe`, `Get`, `Lookup`, `Search`, `BatchGet`,
`Query`, `Scan`, `Filter`, `Head`, `Check`, `Simulate`, `Validate` or `Estimate` is a read.
Everything else is a write. Both come straight from the AWS SDK, so new operations appear when the
SDK is updated.

Some operations are never callable because they return credentials or key material:
for example `secretsmanager.GetSecretValue`, `sts.AssumeRole`, `kms.Decrypt` and `ecr.GetAuthorizationToken`.
Reads that would reveal secrets when a flag is set (`WithDecryption`, `IncludeValue`) reject that flag.
Operations that stream bodies (such as `s3.GetObject`) are not exposed. For object contents, use
a presigned-URL tool or the AWS CLI.

## Replacing the official AWS MCP Server

| Official server | In aws-mcp-turbo |
| --- | --- |
| `aws___run_script` for reads | `aws_query` with `all`, `fields`, `group`, `queries` and recipes. No script is written, so no tokens go on code. |
| `aws___run_script` for writes | `aws_mutate`: preview first, then approval. |
| `aws___get_tasks`, `aws___get_presigned_url` | Not provided. |
| `aws___search_documentation`, `aws___read_documentation`, `aws___retrieve_skill`, `aws___list_regions`, `aws___get_regional_availability` | Not provided. Keep the official server connected for documentation if you use it. |
| Runs remotely, signs through a proxy | Runs locally on your credential chain. |

If you use both servers, their tool names do not collide.

## Configuration and safety

Authentication uses the [AWS SDK default chain](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html):
environment credentials, shared profiles, SSO and role providers, and workload credentials.
`--profile` and `--region` override the defaults. `aws_query` can read from other regions per call
with `region` or `regions`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--profile` | SDK default | Shared AWS profile |
| `--region` | SDK default | Default region |
| `--timeout` | `30s` | Deadline for each query, recipe or approved mutation |
| `--max-output-bytes` | `32768` | Tool output ceiling, from 1 KiB to 1 MiB |
| `--enable-mutations` | `false` | Enable audited previews and the approval flow |
| `--audit-file` | unset | Required private JSONL audit file when mutations are enabled |
| `--approval-hook` | unset | Absolute executable path required for actual writes |
| `--version` | | Print the version and exit |
| `--print-tool-schema` | | Print the tool definitions as JSON |

Writes are off by default. When on, a call still previews unless `execute:true`, and execution
needs the approval hook to approve the exact request. Preview makes **no AWS call** and does not
prove IAM permission. Read [mutation controls and threat model](SECURITY.md) before enabling them.

Give the server a least-privilege identity. The [example IAM policy](examples/iam-readonly.json)
grants broad reads and denies the secret-bearing ones. Logs, object names, tags and custom
projections can still expose sensitive data to the connected model.

## Development and releases

```sh
make check                 # tests, vet, formatting, npm wrapper tests
make race                  # concurrency checks
make build                 # bin/aws-mcp-turbo
node scripts/stdio-smoke.mjs
make bench                 # local CPU and allocation measurements
```

The release binary is about 190 MB because it contains 76 AWS service clients; releases also ship
a gzip copy (about 40 MB) that the npm launcher downloads.

- [Architecture and design decisions](docs/architecture.md)
- [Benchmarks](docs/benchmarks.md) and [ten read-only questions to compare with another server](docs/comparison.md)
- [Operational troubleshooting](docs/operations.md)
- [Maintainer release procedure](docs/releasing.md)
- [Contribution guide](CONTRIBUTING.md), [security policy](SECURITY.md), [changelog](CHANGELOG.md)

MIT licensed. Not affiliated with or endorsed by Amazon Web Services.
