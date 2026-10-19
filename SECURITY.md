# Security policy

Report vulnerabilities privately through the repository's GitHub Security Advisories when
private reporting is enabled. If unavailable, open an issue requesting a private contact
without disclosing exploit details or sensitive data. Never attach credentials, production logs,
or unredacted audit files. Security fixes target the latest release.

## Trust model

This is a local, single-user stdio process. The configured MCP client and operating-system user
are trusted. It is not a multi-tenant gateway and must not be exposed through an unauthenticated
network bridge. Session IDs isolate polling state; they do not authenticate AWS callers.

AWS IAM remains the authorization boundary. Only operations of the 76 bundled AWS SDK service
clients can be called; there is no shell, CLI subprocess or generic HTTP dispatcher. Whether an
operation is a read is **inferred from its name**: `List`, `Describe`, `Get`, `Lookup`, `Search`,
`BatchGet`, `Query`, `Scan`, `Filter`, `Head`, `Check`, `Simulate`, `Validate`, `Estimate`, `Decode`,
`Preview`, `AdminGet` and `AdminList` are reads; everything else, and `sts.GetSessionToken`, is a write.
This is a convention AWS follows consistently but does not guarantee, so grant the server an IAM
identity that could not do harm even if one read-named operation had side effects. Custom JMESPath
can reveal fields omitted by default views, including Lambda environment values. Read access is not synonymous with non-sensitive
access. AWS resource names, logs, and tool output are untrusted data and may contain prompt injection.

The SDK resolves credentials locally and signs requests. This application does not print credential
values. Existing SDK endpoint overrides, proxy variables, and credential-process configuration are
trusted administrator inputs; review them when deploying on a shared machine.

## Operations that are never callable

Operations that return credentials, session tokens or key material are removed entirely, even
for approved writes: `secretsmanager.GetSecretValue`, `secretsmanager.BatchGetSecretValue`,
`ec2.GetPasswordData`, `sts.GetSessionToken`, `sts.GetFederationToken`, `sts.AssumeRole*`,
`ecr.GetAuthorizationToken`, `ecr.GetDownloadUrlForLayer`, `redshift.GetClusterCredentials*`,
`kms.Decrypt`, `kms.GenerateDataKey*`, `iam.CreateAccessKey`, `iam.CreateServiceSpecificCredential`,
`iam.ResetServiceSpecificCredential`, `cognito.GetCredentialsForIdentity` and `cognito.GetOpenIdToken`.
Operations whose input or output is a raw byte stream (such as `s3.GetObject` and `s3.PutObject`)
are not exposed. Reads reject `WithDecryption=true`, `IncludeValue(s)=true`, `IncludePlaintext=true` and
`HidePassword=false`. The deny list is in `internal/turbo/dynamic.go`; extend it when you find an
operation that exposes a secret.

## Mutation gates

Actual execution requires every gate:

1. Server started with `--enable-mutations --audit-file /private/path/audit.jsonl`.
2. The action is an SDK operation that is not a read (and not on the deny list below) and has valid local SDK input fields.
3. Caller supplies a nonblank `intent` and `execute:true`.
4. The full request is successfully appended and synced to the private audit file.
5. A configured approval hook returns `{"approved":true}` and exits 0.
6. Approval is audited successfully before the SDK call.

Example launch (paths are examples and must exist):

```sh
aws-mcp-turbo --profile maintenance --region us-east-1 \
  --enable-mutations --audit-file /private/ops/aws-mcp-audit.jsonl \
  --approval-hook /private/ops/approve-aws-change
```

Example tool arguments:

```json
{"action":"ec2.StopInstances","params":{"InstanceIds":["i-0123456789abcdef0"]},"intent":"Stop the approved test instance after validation","execute":false}
```

Preview is a local plan only. It does not invoke an AWS DryRun parameter, validate resource
existence, prove IAM permission, or reserve state. The actual write may still fail or affect
state that changed after approval. SDK retries remain enabled; maintainers must evaluate
idempotency before adding write actions.

## Human approval hook contract

The hook is an absolute executable path fixed by the administrator. It is invoked directly,
without a shell and without caller-supplied executable arguments. stdin receives one JSON object:

```json
{"time":"2026-01-01T00:00:00Z","session":"server-session-id","status":"requested","request":{"action":"ec2.StopInstances","params":{"InstanceIds":["i-example"]},"intent":"Approved maintenance","execute":true}}
```

The hook must display the exact action, intent, account context configured by the operator,
and parameters through a trusted UI, obtain human approval, and write only
`{"approved":true}` to stdout. No terminal prompt on MCP stdin is possible: stdin belongs to
the JSON request. Implement the hook using your desktop approval UI or internal approval service.
No permissive example hook is shipped. Protect its path against modification by untrusted users.

All other outputs, nonzero exits, missing hooks, and timeouts deny execution. Hook output is
limited to 1 KiB. The overall mutation deadline includes approval and AWS execution; increase
`--timeout` if a human needs longer. The server can enforce the hook contract, but cannot prove
that an administrator's hook actually consulted a human. `execute:true` alone never bypasses it.

## Audit and failure handling

The audit is JSONL, mode 0600, append-only at the application level. Existing non-regular files,
symlinks, or group/world-accessible files are rejected. Records contain exact tool parameters
and caller intent, session identifier, timestamps, and requested/approved/denied/completed/error
status. They may contain secrets. Access, retention, encryption, rotation, and tamper protection
are the operator's responsibility; this is not a tamper-evident compliance ledger.

Audit failures before execution fail closed. If AWS returns but the final audit or output fails,
the tool warns that the write was attempted/succeeded. Check AWS state before retrying. A process
crash or network timeout can leave an uncertain outcome; the tool provides no distributed
transaction or exactly-once mutation guarantee. Restart to reopen a rotated audit file.

## Supply chain

Go dependency versions/checksums are committed; CI runs `go mod verify` and `govulncheck`.
Release builds disable CGO, publish SHA-256 manifests, and attach GitHub provenance attestations.
The npm launcher downloads only versioned GitHub assets and checks their hashes. Checksums from
the same release detect corruption; independent provenance verification adds stronger assurance.
macOS binaries are not Apple-notarized. Use source builds if your environment requires them.
