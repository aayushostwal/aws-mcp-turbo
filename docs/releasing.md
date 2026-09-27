# Maintainer release procedure

The repository includes CI and release automation, but does not automatically publish a public
release or npm package on a branch push. The tag workflow creates a **draft** GitHub release.

1. Finish [operational validation](operations.md), update `CHANGELOG.md`, and set the intended
   version in `npm/package.json`. Stable tags use `vX.Y.Z`.
2. Run `make check`, `make race`, `make build`, `node scripts/stdio-smoke.mjs`, and token metrics.
   Run `make vuln` with the pinned Go toolchain. The scanner version is centralized in
   `Makefile`; review it whenever upgrading Go so its analyzer supports the new language version.
3. Optionally dry-run artifacts with `node scripts/release.mjs v0.1.0` (version must match).
   This cross-compiles macOS/Linux amd64/arm64, writes SHA-256 sums, and generates a stable
   Homebrew formula. It does not publish anything.
4. Create and push a signed version tag when authorized. GitHub Actions checks the code,
   builds assets, attaches provenance, and creates a draft release. Never move a released tag.
5. Review the draft's notes and binaries, then publish the GitHub release. Add known limits,
   benchmark evidence, Go/toolchain details, and the tested client versions.
6. Copy the generated release `aws-mcp-turbo.rb` to `Formula/aws-mcp-turbo.rb` through a PR.
   Verify Homebrew on macOS/Linux before advertising stable tap installation.
7. Publish `npm/` only after the matching GitHub release is public. Configure npm trusted
   publishing or authenticate as the package owner, inspect `npm pack --dry-run`, and run
   `npm publish --access public` from `npm/`. No npm token or auto-publishing credential is
   committed or required by this repository's workflows.
8. Test `npx -y @aayushostwal/aws-mcp-turbo@<version> --version` from a clean cache.

GitHub's release workflow requires contents-write, id-token, and attestation permissions;
ordinary CI has read-only repository permissions. Configure branch protection to require CI,
review dependency updates, and enable private vulnerability reporting where available.
These repository settings are administrative choices, not files this project can enforce.

The wrapper and release builder reject prerelease version strings for now. Publish stable
semver versions only or extend both validators with tests. Windows, Apple notarization, SBOM
publication, automated npm trusted publishing, and remote-proxy latency comparison are follow-ups.
