# Benchmarks and acceptance status

Measured locally on 2026-10-18, Apple M1, macOS arm64, Go 1.27.2, MCP Go SDK 1.8.0.
These are offline measurements on synthetic, SDK-shaped payloads. No AWS account was contacted
and no other MCP server was run. Where a result needs one of those, it is marked **unverified**.

## Results

| Claim | Measured result | Status |
| --- | --- | --- |
| At most three tools | Exactly three | Met |
| Tool definitions under 500 tokens | 272 (`cl100k_base`), 289 (`o200k_base`) | Met for these tokenizers |
| Server overhead under 50 ms | About 0.5 ms per query on 1,000 synthetic instances, projection through rendering | Met for this benchmark's scope |
| Startup | About 40 ms to print `--version` on a warm binary; services are indexed only when used | Measured |
| Single-record responses reduced 85% | 45% to 95%, mean 78% over ten one-record fixtures | **Not met** |
| Listings of 25 to 40 records reduced 85% | 90% to 97% over four listing fixtures | Met on these fixtures |
| Faster and cheaper per task than the official AWS MCP Server | No comparison run | **Unverified**; the harness is in `scripts/bench-compare.mjs` |
| Claude Desktop and Cursor integration | Real stdio smoke passes; no GUI run | GUI acceptance unverified |

Single records are the weak case because there is little to remove: a security group's rules or
an ECS task's stop reason are the answer. The gain comes from lists, which are what agents
mostly read. Default views for generated actions are heuristics; they were checked on
representative operations (`TestDefaultProjectionsShowWhatMatters`), not on every one of about
2,700 reads.

## Payload tokens

`testdata/payloads.json` holds hand-written, SDK-shaped responses: ten single-record fixtures and
four listings built by repeating a record with varied identifiers. They are not claimed to represent
the most frequent APIs or any real account. The raw baseline is compact JSON, not pretty-printed.
The compact side is the real engine's output, including paging text.

| Fixture | Raw tokens | Tool tokens | Reduction (`cl100k_base`) |
| --- | ---: | ---: | ---: |
| `ec2.DescribeInstances` | 607 | 30 | 95.1% |
| `ec2.DescribeVolumes` | 229 | 42 | 81.7% |
| `ec2.DescribeSecurityGroups` | 175 | 97 | 44.6% |
| `s3.ListBuckets` | 145 | 34 | 76.6% |
| `s3.ListObjectsV2` | 172 | 27 | 84.3% |
| `lambda.ListFunctions` | 239 | 33 | 86.2% |
| `lambda.GetFunctionConfiguration` | 282 | 29 | 89.7% |
| `ecs.DescribeTasks` | 427 | 73 | 82.9% |
| `logs.GetLogEvents` | 129 | 48 | 62.8% |
| `logs.FilterLogEvents` | 121 | 33 | 72.7% |
| `ec2.DescribeInstances` x25 | 14,775 | 521 | 96.5% |
| `lambda.ListFunctions` x30 | 6,857 | 499 | 92.7% |
| `s3.ListObjectsV2` x40 | 4,152 | 348 | 91.6% |
| `ec2.DescribeVolumes` x25 | 5,305 | 513 | 90.3% |

The previous release measured 71% mean over the ten single-record fixtures; the same ten now
average 78%. Security-group rules and diagnostic evidence are kept even where this limits savings.
Other savings are not in these numbers: `all` replaces a loop of calls, `queries` and recipes
replace several calls with one, `group` returns counts instead of rows, and polling skips events
already shown. They depend on the workload.

## Comparing with another server

`scripts/bench-compare.mjs` runs the same twelve questions from `bench/tasks.json` against this
server and another one over stdio, using your AWS credentials (every task is a read). For the
official AWS MCP Server, launch its proxy as the other server:

```sh
node scripts/bench-compare.mjs --turbo ./bin/aws-mcp-turbo \
  --other "uvx mcp-proxy-for-aws-cli@latest https://aws-mcp.us-east-1.api.aws/mcp --metadata AWS_REGION=us-west-2" \
  --var REGION_A=us-east-1 --var REGION_B=us-west-2 --var REGION_C=eu-west-1 \
  --var FUNCTION=my-function --var CLUSTER=my-cluster --out bench/results.json
.venv/bin/python scripts/token_metrics.py --compare bench/results.json
```

It records tool-definition size, the arguments and response of every call, and wall time, then
counts tokens per task. The reference calls assume the smallest sensible request on each side: for
the official server, a short boto3 script that prints only the fields the question needs. The
`aws___run_script` argument name in `bench/tasks.json` (`script`) is a guess: AWS's documentation does
not list the schema, so check it against the `tools/list` output and fix the file if it differs.

This measures what a well-driven agent would send. It does not measure how many attempts a real model
needs, which favors whichever server is easier to use correctly. Treat the result as a lower bound
for both. Publish the transcripts with any claim.

## Reproduce

```sh
make bench
python3 -m venv .venv
.venv/bin/pip install -r scripts/requirements.txt
.venv/bin/python scripts/token_metrics.py --check-schema
```

The script runs the real binary's schema export and the fixture measurement, then counts
`cl100k_base` and `o200k_base` tokens. The first run downloads the tokenizer vocabulary.
`--check-schema` fails at 500 tokens. `--check-savings` enforces the 85% mean and currently fails
on the single-record fixtures; it is deliberately not part of CI.

## Still required before calling a release production-ready

Run the harness above against a named remote server with identical identities, regions and page
sizes, and report p50 and p95. Separately time cold start, credential refresh, AWS round trip and warm
server work. Test SSO refresh, late log arrivals, a real approval UI, and the supported GUI client
versions. Run the read tools against a real account: this release's AWS calls were checked against
stubbed HTTP responses, not a live service.
