# @ostwal/aws-mcp-turbo

Node launcher for the native [aws-mcp-turbo](https://github.com/aayushostwal/aws-mcp-turbo)
MCP server. Requires Node 22+, macOS/Linux, and arm64/x64.

Run without cloning the repository or installing Go:

```sh
npx -y @ostwal/aws-mcp-turbo@latest --version
```

Configure an MCP client to launch this command with `--region <region>`.
It exposes three tools for AWS discovery, compact reads across 76 services (with batching and recipes), and guarded writes.
Mutations are disabled by default. Credentials use the standard AWS SDK chain; SSO login is
performed separately using the AWS CLI.

On first use, the wrapper downloads the matching native GitHub release (a gzip copy of about 40 MB) and verifies SHA-256 of both the download and the binary.
It caches the executable in `$XDG_CACHE_HOME/aws-mcp-turbo/<version>` or
`~/.cache/aws-mcp-turbo/<version>`, and verifies the cached hash before execution.
Download diagnostics use stderr; stdin/stdout belong to MCP. Use `@latest` in IDE configs to
follow new releases on restart, or pin a version if reproducibility is more important.

For local/offline development, set `AWS_MCP_TURBO_BINARY` to an absolute executable path.
This explicit override skips download/hash verification. The wrapper has no npm runtime dependencies.

See the repository's [client guide](https://github.com/aayushostwal/aws-mcp-turbo/blob/main/docs/clients.md),
[security policy](https://github.com/aayushostwal/aws-mcp-turbo/blob/main/SECURITY.md), and
[tool reference](https://github.com/aayushostwal/aws-mcp-turbo/blob/main/docs/tools.md).
