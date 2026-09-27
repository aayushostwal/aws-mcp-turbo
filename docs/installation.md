# Installation

## Source

Use the Go version declared in `go.mod` or newer. Go's automatic toolchain selection can
download the pinned toolchain on first build. No C compiler is needed for the release binary.

```sh
git clone https://github.com/aayushostwal/aws-mcp-turbo.git
cd aws-mcp-turbo
make build
./bin/aws-mcp-turbo --version
```

Use the resulting absolute binary path in your MCP client. Alternatively, once a version is
published, `go install github.com/aayushostwal/aws-mcp-turbo/cmd/aws-mcp-turbo@v0.1.0`
builds into Go's binary directory; source installs report `dev` unless built with version ldflags.

## GitHub binary releases

After a release is published, select your asset from
[GitHub Releases](https://github.com/aayushostwal/aws-mcp-turbo/releases):

| Platform | Asset suffix |
| --- | --- |
| Apple Silicon macOS | `darwin_arm64` |
| Intel macOS | `darwin_amd64` |
| ARM Linux | `linux_arm64` |
| x86-64 Linux | `linux_amd64` |

Example for Apple Silicon, when `v0.1.0` is available:

```sh
gh release download v0.1.0 --repo aayushostwal/aws-mcp-turbo \
  --pattern aws-mcp-turbo_v0.1.0_darwin_arm64 --pattern checksums.txt
shasum -a 256 aws-mcp-turbo_v0.1.0_darwin_arm64
```

Compare the digest with the matching `checksums.txt` entry. Optionally verify provenance:

```sh
gh attestation verify aws-mcp-turbo_v0.1.0_darwin_arm64 --repo aayushostwal/aws-mcp-turbo
chmod +x aws-mcp-turbo_v0.1.0_darwin_arm64
./aws-mcp-turbo_v0.1.0_darwin_arm64 --version
```

Copy it to an installation directory you control, or use this path directly in your client.
macOS signing/notarization and Windows binaries are not currently provided.

## Homebrew

The repository itself can serve as a tap. Before a stable formula is published, build HEAD:

```sh
brew tap aayushostwal/aws-mcp-turbo https://github.com/aayushostwal/aws-mcp-turbo.git
brew install --HEAD aayushostwal/aws-mcp-turbo/aws-mcp-turbo
```

Each tagged release generates `aws-mcp-turbo.rb` with exact checksums for all four binaries.
Maintainers copy that generated file into `Formula/aws-mcp-turbo.rb` after publishing the
release to enable normal stable `brew install aayushostwal/aws-mcp-turbo/aws-mcp-turbo`.

## npm

With Node.js 22+ installed, run:

```sh
npx -y @ostwal/aws-mcp-turbo@0.1.0 --version
```

The wrapper has no runtime npm dependencies. It requires Node 22+ and downloads the native
binary from the matching GitHub release, verifies SHA-256, and caches it under
`$XDG_CACHE_HOME/aws-mcp-turbo/<version>` or `~/.cache/aws-mcp-turbo/<version>`.
Cached binaries are hashed before execution. First use requires network access; later use
works offline with an intact cache. Checksum failures prevent execution.

Set `AWS_MCP_TURBO_BINARY` to an absolute local executable to bypass downloading, for example
when testing the wrapper before publication. This is an explicit administrator override and
does not verify the chosen local binary. npm is only a launcher; AWS requests run in Go.
