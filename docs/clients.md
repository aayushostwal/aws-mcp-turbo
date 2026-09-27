# Client configuration

Use an absolute binary path. GUI applications often have a different `PATH` from a terminal.
The client launches the process and exchanges newline-delimited JSON-RPC over stdin/stdout.
Application logs go to stderr. Every client-launched process owns its own cache and AWS configuration.

## Claude Desktop

Open Settings → Developer → Edit Config and merge the following entry into `mcpServers`:

```json
{
  "mcpServers": {
    "aws-turbo": {
      "command": "/absolute/path/to/aws-mcp-turbo",
      "args": ["--profile", "development", "--region", "us-east-1"]
    }
  }
}
```

On macOS this is normally `~/Library/Application Support/Claude/claude_desktop_config.json`.
Restart the client, verify four tools appear, and ask it to discover EC2 actions.
Configuration follows the [MCP local-server guide](https://github.com/modelcontextprotocol/docs/blob/main/quickstart/user.mdx).
The project tests generic MCP interoperability; each GUI version still needs a manual smoke test.

## Cursor

Use the same JSON in `.cursor/mcp.json` for the project or `~/.cursor/mcp.json` for the user.
Enable the server in MCP settings and check its status. Avoid committing machine-specific
paths or credentials in shared project configuration. See [Cursor's MCP documentation](https://docs.cursor.com/context/model-context-protocol).

## Generic MCP clients

Configure a stdio server with a command, argument array, and optional environment. The official
Go SDK handles protocol negotiation, initialization, cancellation, and tool calls. No listener
port, HTTP endpoint, OAuth gateway, or remote transport is provided.

For SDK environments that use environment variables instead of flags:

```json
{
  "mcpServers": {
    "aws-turbo": {
      "command": "/absolute/path/to/aws-mcp-turbo",
      "env": {"AWS_PROFILE":"development","AWS_REGION":"us-east-1"}
    }
  }
}
```

## npm wrapper (after publication)

```json
{
  "mcpServers": {
    "aws-turbo": {
      "command": "npx",
      "args": ["-y", "@aayushostwal/aws-mcp-turbo@0.1.0", "--profile", "development", "--region", "us-east-1"]
    }
  }
}
```

Node 22+ is required only for this launcher. Its first run downloads the matching native binary
and SHA-256 manifest; subsequent runs verify and reuse the local cache. Installation/download
can exceed a client's startup timeout, so run `npx ... --version` in a terminal first.
Use `AWS_MCP_TURBO_BINARY=/absolute/path/to/binary` for offline/local package testing.

## Multiple accounts

Create separate server entries (`aws-dev`, `aws-prod`) with separate profiles. Use a read-only
production role and make each entry's name explicit. A single running process has one fixed
AWS configuration; tool arguments cannot select another profile or region.
