# Contributing

Open a focused issue or pull request describing the problem, expected behavior, and validation.
Use the Go version in `go.mod`; Node 22+ tests the optional launcher. Python/tiktoken is only
needed for token measurements. Unit/integration tests must not require AWS credentials.

```sh
make check
make race
make build
node scripts/stdio-smoke.mjs
make bench
```

For a new action:

1. Add a typed SDK binding in `internal/turbo/aws.go` and explicitly classify read vs write.
2. Supply a compiled projection that preserves useful operational evidence, required fields,
   and pagination token mappings. Do not remove security rules or errors to inflate savings.
3. Test input validation, projected output, pagination and failures with fixtures or a fake
   HTTP transport. Include write-gate tests for mutations.
4. Update discovery behavior as needed, supported-action docs, and the example IAM policy.
5. Add realistic sanitized fixtures to token measurements and report the impact honestly.

Keep the exposed tool count at four and measured definitions below 500 tokens. Avoid additional
protocol output, raw SDK dumps, implicit profile switching, and unbounded page aggregation.
Comments should explain non-obvious constraints, especially cursor and approval semantics.

Do not commit credentials, AWS response dumps from real accounts, local audit JSONL, built
binaries, caches, or generated coverage. Benchmark fixtures must remain synthetic or explicitly
sanitized and reviewed. Report security issues according to [SECURITY.md](SECURITY.md).
