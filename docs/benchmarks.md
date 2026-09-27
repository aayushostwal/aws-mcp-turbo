# Benchmarks and acceptance status

Measured locally on 2026-09-27, Apple M4, macOS arm64, Go 1.27.1, MCP Go SDK 1.8.0.
These are offline synthetic measurements, not live AWS or production fleet results.

## Results

| Requirement | Measured result | Status |
| --- | --- | --- |
| At most four tools | Exactly four | Met |
| Tool definitions under 500 tokens | 258 (`cl100k_base`), 271 (`o200k_base`), including annotations | Met for these tokenizers |
| Internal overhead under 50 ms | ~0.57 ms/op for projection/pruning/formatting of 1,000 synthetic instances | Met for this benchmark scope |
| Average 85% response token reduction | 71.1% / 71.5% unweighted mean across ten fixture actions | **Not met** |
| 3–5× faster than remote proxy | No controlled remote comparison performed | Unverified |
| Claude Desktop / Cursor integration | Generic SDK integration and real stdio smoke pass | GUI acceptance unverified |

`BenchmarkQueryPipeline` includes engine routing, parameters, JMESPath, pruning, formatting,
and bounds. It excludes AWS network/credential work, SDK output normalization, transport
delivery, and startup. `BenchmarkProjectionFormatting` measures pruning/formatting alone:
~0.32 ms/op. The pipeline allocated ~1.0 MB/op for 1,000 rows on this machine. Timings vary
with runtime, hardware, row shape, and load; CI does not enforce a flaky wall-clock threshold.

## Payload tokens

The fixtures in `testdata/payloads.json` are hand-authored, synthetic, SDK-shaped responses.
They are not claimed to represent the ten most frequent APIs or all production accounts.
The raw baseline is compact JSON, not pretty-printed JSON. The compressed side uses the real
engine, including polling/pagination text. Security-group rules and diagnostic evidence are
retained even where this limits savings.

| Action | Raw tokens | Tool tokens | Reduction (`cl100k_base`) |
| --- | ---: | ---: | ---: |
| `ec2.DescribeInstances` | 607 | 48 | 92.1% |
| `ec2.DescribeVolumes` | 229 | 63 | 72.5% |
| `ec2.DescribeSecurityGroups` | 175 | 114 | 34.9% |
| `s3.ListBuckets` | 145 | 48 | 66.9% |
| `s3.ListObjectsV2` | 172 | 45 | 73.8% |
| `lambda.ListFunctions` | 239 | 50 | 79.1% |
| `lambda.GetFunctionConfiguration` | 282 | 31 | 89.0% |
| `ecs.DescribeTasks` | 427 | 73 | 82.9% |
| `logs.GetLogEvents` | 129 | 57 | 55.8% |
| `logs.FilterLogEvents` | 121 | 44 | 63.6% |

Small/already-flat responses and message-heavy logs have lower compressibility. Polling can
save much more over a session by not replaying history, but that is a separate workload-dependent
benefit and is not added to these per-response figures. Custom projections can reduce output
further by requesting fewer fields; no percentage is guaranteed for arbitrary projections.

## Reproduce

```sh
make bench
python3 -m venv .venv
.venv/bin/pip install -r scripts/requirements.txt
.venv/bin/python scripts/token_metrics.py --check-schema
```

The script runs the real binary's schema export and a fixture measurement command, then counts
both `cl100k_base` and `o200k_base` tokens. Initial tokenizer vocabulary download needs network
access; the measurements never contact AWS. `--check-schema` fails at 500 tokens.
`--check-savings` enforces the aspirational 85% threshold and currently fails; it is intentionally
not used as a passing CI claim. Add sanitized workload fixtures before reassessing the target.

## Validation still required before a production release

Run identical IAM identities, regions, queries, page sizes, and payloads against this binary
and a named remote-proxy implementation. Separate cold startup, credential refresh, AWS RTT,
and warm server CPU costs; report p50/p95 and token counts for repeated workflows. Also test
SSO refresh, late log arrivals, real approval UI behavior, and supported GUI client versions.
Release artifacts and documentation are prepared; these acceptance gates are not waived.
