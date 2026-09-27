# aws-mcp-turbo

[![CI](https://github.com/aayushostwal/aws-mcp-turbo/actions/workflows/ci.yml/badge.svg)](https://github.com/aayushostwal/aws-mcp-turbo/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A native Go MCP server that keeps AWS responses small before they reach your agent.
Four tools provide discovery, projected reads, diagnostic macros, and explicitly approved writes.
It runs locally over stdio and uses your normal AWS credential chain.

**Status:** initial implementation. Source installation works; binary releases and the npm package
become available after a maintainer publishes them. Offline tests cover the protocol, AWS request
encoding, projections, cache behavior, and mutation gates. Live-account and GUI-client certification
are still release checks. Performance targets are tracked in [benchmarks](docs/benchmarks.md), not advertised as guarantees.

## Why it exists

An agent usually needs an instance's identity, state, and address—not every field of
`DescribeInstances`. `aws-mcp-turbo` projects SDK responses through JMESPath, removes absent
data, and renders lists as compact Markdown or TSV. Tool definitions stay small because
service-specific arguments and presets are discovered only when needed.

```text
Agent / IDE ── stdio MCP ── aws-mcp-turbo ── AWS SDK v2 ── AWS APIs
                                │
                         projection → pruning → formatting
                         session log cursors / mutation audit + approval
```

There is no application proxy, hosted gateway, telemetry service, or server-side account to create.
AWS calls still incur network latency and normal AWS charges.

## Quick start

Requires the patched Go toolchain declared in `go.mod` for source builds. macOS and Linux,
arm64 and amd64, are supported.

```sh
git clone https://github.com/aayushostwal/aws-mcp-turbo.git
cd aws-mcp-turbo
make build
./bin/aws-mcp-turbo --version
```

Use an existing AWS profile. For an SSO profile, authenticate with the AWS CLI first:

```sh
aws sso login --profile development
./bin/aws-mcp-turbo --profile development --region us-east-1
```

The running server waits for MCP messages on stdin; an idle terminal is expected.
Connect an MCP client to use it. No AWS credentials are needed for discovery or local tests.

Add this configuration to your client, replacing the binary path and profile:

```json
{
  "mcpServers": {
    "aws-turbo": {
      "command": "/absolute/path/aws-mcp-turbo/bin/aws-mcp-turbo",
      "args": ["--profile", "development", "--region", "us-east-1"]
    }
  }
}
```

See [client configuration](docs/clients.md) for Claude Desktop, Cursor, generic stdio clients,
and npm. See [installation](docs/installation.md) for verified binaries and Homebrew.

## Four tools

| Tool | Purpose | Default behavior |
| --- | --- | --- |
| `aws_discover` | Find actions, presets, required parameters, and macros | Local only; exact action names expose full parameter names/types |
| `aws_query` | Run a registered read operation | One AWS page, preset projection, Markdown lists |
| `aws_diagnose` | Gather ECS stop or Lambda timeout evidence | Read only, bounded composite calls |
| `aws_mutate` | Preview or execute a registered write | Disabled; enabled calls still preview unless `execute:true` |

Examples below are tool arguments, not shell commands.

Discover EC2 actions:

```json
{"search":"ec2"}
```

Query instances using a built-in projection:

```json
{"action":"ec2.DescribeInstances","params":{"Filters":[{"Name":"instance-state-name","Values":["running"]}]}}
```

Example output:

```text
|Id|Name|PrivateIp|State|Type|
|---|---|---|---|---|
|i-0123456789abcdef0|api|10.0.1.10|running|t3.medium|
```

Override the projection or choose TSV/JSON:

```json
{"action":"s3.ListObjectsV2","params":{"Bucket":"example-artifacts","MaxKeys":100},"projection":"Contents[].{Key:Key,Bytes:Size}","format":"tsv"}
```

Poll a log stream using identical arguments to get subsequent pages/new events:

```json
{"action":"logs.GetLogEvents","params":{"LogGroupName":"/aws/lambda/api","LogStreamName":"2026/01/01/[$LATEST]example","StartTime":1767225600000}}
```

Collect evidence:

```json
{"macro":"ecs_task_stopped","params":{"Cluster":"production"}}
```

```json
{"macro":"lambda_timeout","params":{"FunctionName":"api-handler"}}
```

The complete [tool reference](docs/tools.md) documents parameter casing, pagination,
cache lifetime, errors, custom projections, and diagnostic limits.

## Supported actions

| Service | Reads | Writes (opt-in) |
| --- | --- | --- |
| EC2 | `DescribeInstances`, `DescribeVolumes`, `DescribeSecurityGroups` | `StartInstances`, `StopInstances` |
| S3 | `ListBuckets`, `ListObjectsV2`, `GetBucketLocation` | — |
| Lambda | `ListFunctions`, `GetFunctionConfiguration` | `UpdateFunctionConfiguration` |
| ECS | `ListTasks`, `DescribeTasks` | — |
| CloudWatch Logs (`logs`) | `GetLogEvents`, `FilterLogEvents` | — |

This is an explicit allowlist, not a universal AWS API dispatcher. New actions require
reviewed SDK bindings, safety classification, projections, and tests.

## Configuration and safety

Authentication uses the [AWS SDK default chain](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html):
environment credentials, shared profiles, SSO and role providers, and workload credentials
where configured. `--profile` and `--region` override SDK defaults. Separate server entries
can use different accounts/regions. The MCP caller cannot change either per request.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--profile` | SDK default | Shared AWS profile |
| `--region` | SDK default | AWS region |
| `--timeout` | `30s` | Deadline for each query, macro, or approved mutation |
| `--max-output-bytes` | `32768` | Tool output ceiling; configurable from 1 KiB to 1 MiB |
| `--enable-mutations` | `false` | Enable audited previews and approval flow |
| `--audit-file` | unset | Required private JSONL audit file when mutations enabled |
| `--approval-hook` | unset | Absolute executable path required for actual writes |
| `--version` | — | Print version and exit |
| `--print-tool-schema` | — | Emit the complete four-tool definition JSON |

Mutations require an explicit intent and an administrator-configured hook that approves the
exact request. `execute:true` is a request for approval, never approval itself. Preview mode
makes **no AWS call** and does not prove IAM authorization or AWS-side validation.
See [mutation controls and threat model](SECURITY.md) before enabling writes.

Use a least-privilege AWS identity. [Example IAM policy](examples/iam-readonly.json) covers
the shipped reads and must be tailored to your accounts and resources. Logs, object names,
and custom projections can expose sensitive data to the connected model.

## Development and releases

```sh
make check                 # tests, vet, formatting, npm wrapper tests
make race                  # concurrency checks
make build                 # bin/aws-mcp-turbo
node scripts/stdio-smoke.mjs
make bench                 # local CPU/allocation measurements
```

CI runs on Linux and macOS, scans Go vulnerabilities, checks the schema token budget,
and exercises a real stdio handshake. Tagged releases build four native binaries,
SHA-256 checksums, provenance attestations, and a Homebrew formula, then create a draft release.

- [Architecture and design decisions](docs/architecture.md)
- [Benchmarks and acceptance status](docs/benchmarks.md)
- [Operational troubleshooting](docs/operations.md)
- [Maintainer release procedure](docs/releasing.md)
- [Contribution guide](CONTRIBUTING.md), [security policy](SECURITY.md), [changelog](CHANGELOG.md)

MIT licensed. Not affiliated with or endorsed by Amazon Web Services.
