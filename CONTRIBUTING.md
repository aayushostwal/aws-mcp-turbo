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

Actions come from the AWS SDK clients, so most changes are not new actions:

- **A new service:** add one line to `internal/turbo/services.go` (and `go get` the SDK module).
  `TestEveryOperationHasUsableDefaults` checks that every operation gets a valid default view.
- **A better default for one action:** add an entry to `internal/turbo/presets.go` with a projection that
  keeps useful operational evidence, required fields, and a `Root` when `fields` should apply somewhere
  other than the default. `TestPresetsPointAtRealOperations` checks it against the SDK types.
- **A secret-bearing operation or flag:** extend the deny list or `guardedFlags` in `dynamic.go`, with a test.
- **A recipe:** add an entry to `internal/turbo/recipes.go`; `TestEveryRecipeActionExistsWithItsParameters`
  checks every step against the SDK.

Test input validation, output, pagination and failures with fixtures or a fake HTTP transport
(see `features_test.go`). Add realistic sanitized fixtures to token measurements and report the
impact honestly, and do not remove security rules or errors to inflate savings.

Keep the exposed tool count at three and measured definitions below 500 tokens. Avoid additional
protocol output, raw SDK dumps, implicit profile switching, and unbounded page aggregation.
Comments should explain non-obvious constraints, especially cursor and approval semantics.

Do not commit credentials, AWS response dumps from real accounts, local audit JSONL, built
binaries, caches, or generated coverage. Benchmark fixtures must remain synthetic or explicitly
sanitized and reviewed. Report security issues according to [SECURITY.md](SECURITY.md).
