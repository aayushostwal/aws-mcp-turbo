# Operations and troubleshooting

Run one process per client/profile/region. There is no network listener or daemon management.
The client owns process lifecycle; SIGINT/SIGTERM cancel the server. stdout is reserved for MCP.
Startup errors go to stderr; tool errors are text blocks with `isError:true`.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Server starts but displays nothing | Normal: stdio waits for a client; use the client or `--version` |
| Client cannot launch it | Absolute executable path, executable permission, supported OS/architecture |
| SSO credentials expired | Run `aws sso login --profile <profile>` outside the MCP process |
| Missing/incorrect region | Set `--region` or SDK region configuration |
| Access denied | Verify the intended AWS identity and action/resource IAM grants |
| Unknown parameter | Discover the exact action; use PascalCase Go SDK field names |
| Response too large | Reduce page size or filters; narrow projection; use TSV; raise output ceiling if appropriate |
| Log history repeats | Cache restart/TTL/eviction, changed params, another MCP session, or manual tokens |
| Log query returns no rows | An empty page can still advance; repeat or follow the continuation metadata |
| Mutation disabled | Deliberate default; configure opt-in/audit/hook according to SECURITY.md |
| Hook times out | Approval is included in `--timeout`; use a trusted UI and suitable deadline |
| npm release asset not found | Matching GitHub release must be public and published before the npm package |
| macOS refuses a downloaded binary | Follow organizational trust/notarization policy or build from source |

For identity troubleshooting, use `aws sts get-caller-identity --profile <profile>` in a terminal.
This server does not register STS actions and does not run that command automatically.
Do not paste account-sensitive output into public issues.

## Resource limits and failure semantics

Default request deadline: 30 seconds; SDK maximum attempts: 3. SDK retries consume the same
deadline. JMESPath operates on an in-memory page and has no separately cancellable evaluator;
very large pages or expressions can briefly exceed the deadline during CPU work. Use filters
and sensible AWS page sizes. Output bounds are enforced after projection, not on raw AWS bytes.

Handler input is limited to 64 KiB; output defaults to 32 KiB and can be configured up to 1 MiB.
The Go SDK owns transport buffering. This is not a hardened public service or a per-session
quota system. Keep untrusted clients off the process and monitor local resource usage.

The cache stores at most 256 streams × 16 checkpoints. Inactive data is expired lazily on
cache access; all memory is released when the process ends. No log text is persisted.
Audit files grow until rotated by the operator. Stop/restart the process to reopen a rotated file.

AWS pagination and partial diagnostic errors are explicit. Mutation uncertainty after a network
failure or process crash must be resolved by inspecting AWS state, not blindly repeating writes.

## Release validation checklist

- Run CI and review vulnerability findings with a supported patched Go version.
- Use a dedicated AWS sandbox role for live read/pagination and log-tail tests.
- Verify multiple equal-timestamp log events and a quiet stream followed by new data.
- Verify SSO refresh and intended role assumption in your actual environment.
- Connect the supported Claude Desktop and Cursor versions; verify four tools and clean stdout.
- Test mutations only against disposable resources with a real human approval hook.
- Record the platform, binary version, AWS region, latency distribution, and payload sizes.
- Verify published checksums/attestations and npm/Homebrew installs before announcing support.
