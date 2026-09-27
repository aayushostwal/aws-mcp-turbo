# Maintainer release procedure

Ordinary pushes to `main` and pull requests run CI only. After this workflow is merged and npm
trust is configured, pushing a stable `vX.Y.Z` tag automatically runs the release gates,
publishes GitHub binaries, then publishes `@ostwal/aws-mcp-turbo` to npm. Merging a version bump
does **not** create a tag or publish a version. No stored npm token is used.

## One-time npm setup (package owner)

In [the npm package settings](https://www.npmjs.com/package/@ostwal/aws-mcp-turbo/access),
add a **GitHub Actions trusted publisher** with these exact values:

| Setting | Value |
| --- | --- |
| Organization or user | `aayushostwal` (GitHub owner, not the npm username) |
| Repository | `aws-mcp-turbo` |
| Workflow filename | `release.yml` (not `.github/workflows/release.yml`) |
| Environment name | Leave blank; this workflow does not declare an environment |
| Allowed actions | Enable direct `npm publish`, not only staged publishing |

This npm-side trust entry cannot be established by merging repository files. Configure it
before pushing the first automated release tag. Do not add `NPM_TOKEN` or `NODE_AUTH_TOKEN`
secrets. The publishing job uses GitHub-hosted Ubuntu, `id-token: write`, Node 22 and pinned
npm 11.20.0. See [npm's trusted publishing instructions](https://docs.npmjs.com/trusted-publishers/).
After verifying OIDC publication works, consider restricting traditional publishing tokens in
npm settings. Never put tokens or one-time codes in a PR.

On GitHub, protect `main` with required CI/review and restrict creation/update/deletion of `v*`
tags to release maintainers. Tags authorize public publication; treat tag creation as a release
approval. These are repository-owner settings, not changes this workflow can enforce.

## Cut a release

1. Open a PR updating `npm/package.json`, installation examples, `CHANGELOG.md`, and
   `docs/releases/vX.Y.Z.md`. Complete the applicable [operational validation](operations.md),
   and explicitly record remaining acceptance limits in the release notes.
2. Run `make check`, `make race`, `make vuln`, `make build`, `node scripts/stdio-smoke.mjs`,
   `node scripts/npm-smoke.mjs`, and token metrics. Review CI and merge the PR.
3. Pull `main`, check the version, and create an annotated tag (sign it if configured):

   ```sh
   git switch main
   git pull --ff-only
   node scripts/check-release.mjs v0.1.1
   git tag -a v0.1.1 -m 'aws-mcp-turbo v0.1.1'
   git push origin v0.1.1
   ```

   Substitute the intended version on future releases. The tagged commit must be reachable
   from `main`, the tag must exactly match the package version, and release notes must exist.
   Stable versions only: no prerelease suffixes, build metadata, or leading zeroes.
4. Watch the **Release** workflow. It runs tests/race/vulnerability checks, builds all four
   binaries, verifies hashes and the Linux binary version, attaches attestations, and publishes
   a public GitHub release with the checked-in release notes.
5. The dependent npm job tests a locally packed wrapper against that public binary release,
   then publishes with OIDC and provenance. A final fresh-cache public npm/MCP smoke test waits
   up to 24 registry lookups, with 10-second waits, for visibility. npm package-index E404/ETARGET
   errors get up to 12 attempts; binary/hash failures fail immediately. Individual requests also
   have timeouts. The smoke covers download/hash verification, four tools, discovery, mutation denial,
   offline cached startup, and clean shutdown. A successful upload alone is not acceptance.
6. Verify the workflow is green and run `npx -y @ostwal/aws-mcp-turbo@0.1.1 --version` yourself.
   Launch releases one at a time; workflow concurrency prevents overlapping release runs.

For a local artifact build without publishing, run `node scripts/release.mjs v0.1.1` after
updating the version. Homebrew remains a separate task: copy the generated formula through a
PR and validate it before advertising stable installation. Windows, Apple notarization, SBOM
publication, and remote-proxy performance comparisons remain follow-ups.

## Failures and safe retries

- **Missing npm trust or authentication failure:** correct the npm publisher values above,
  including direct-publish permission. In GitHub Actions, choose **Re-run failed jobs**, which
  retries npm without re-running the successful binary publication job. Do not use `npm whoami`
  as an OIDC test: that command does not use the publishing OIDC exchange.
- **npm succeeded but verification failed:** re-run failed jobs. The publisher compares the
  remote version's SHA-512 integrity with a freshly packed tarball; an identical version is
  skipped and verified again. Conflicting content or lookup/auth/server errors fail closed.
- **GitHub publication failed or all jobs were re-run:** the workflow will not overwrite an
  existing release or clobber assets. Inspect the release and workflow logs before proceeding;
  do not delete/recreate public assets or move a tag to make a rerun pass. An incomplete draft
  requires maintainer review. If the tagged source needs correction, release a new version.
- **A newer npm release is already latest:** an unpublished older version is rejected rather
  than moving `latest` backwards. Never use concurrent/out-of-order tag pushes as a release queue.
- **Bad released behavior:** pin the last validated version (`0.1.0` for this update) in the MCP
  client and restart it, or disable the entry. This stops requests but cannot undo AWS writes.
  Deprecate a defective version with an actionable advisory and publish a separately versioned fix.

Keep matching GitHub binaries available for existing pinned npm versions. Never overwrite
published npm content, release assets, or Git tags. The scanner version is centralized in
`Makefile`; review analyzer compatibility whenever upgrading the Go toolchain.
