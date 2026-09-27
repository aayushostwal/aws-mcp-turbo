# @ostwal/aws-mcp-turbo

Node launcher for the native [aws-mcp-turbo](https://github.com/aayushostwal/aws-mcp-turbo)
MCP server. Requires Node 22+, macOS/Linux, and arm64/x64.

Run without cloning the repository or installing Go:

```sh
npx -y @ostwal/aws-mcp-turbo@0.1.1 --version
```

Configure an MCP client to launch this command with `--profile <aws-profile> --region <region>`.
It exposes four tools for AWS discovery, projected reads, diagnostic macros, and guarded writes.
Mutations are disabled by default. Credentials use the standard AWS SDK chain; SSO login is
performed separately using the AWS CLI.

On first use, the wrapper downloads the matching native GitHub release and verifies SHA-256.
It caches the executable in `$XDG_CACHE_HOME/aws-mcp-turbo/<version>` or
`~/.cache/aws-mcp-turbo/<version>`, and verifies the cached hash before execution.
Download diagnostics use stderr; stdin/stdout belong to MCP. Pin a package version in IDE configs.

For local/offline development, set `AWS_MCP_TURBO_BINARY` to an absolute executable path.
This explicit override skips download/hash verification. The wrapper has no npm runtime dependencies.

See the repository's [client guide](https://github.com/aayushostwal/aws-mcp-turbo/blob/main/docs/clients.md),
[security policy](https://github.com/aayushostwal/aws-mcp-turbo/blob/main/SECURITY.md), and
[tool reference](https://github.com/aayushostwal/aws-mcp-turbo/blob/main/docs/tools.md).
