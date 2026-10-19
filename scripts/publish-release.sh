#!/usr/bin/env bash
# Publish a release: check the repository, make sure npm trusted publishing is
# configured, tag, push the tag, and watch the Release workflow.
#
#   scripts/publish-release.sh 0.2.0          # interactive, asks before tagging
#   scripts/publish-release.sh 0.2.0 --yes    # no confirmation prompt
#   scripts/publish-release.sh 0.2.0 --setup-only   # only configure npm, do not tag
#
# Pushing the tag is what publishes. Everything before it can be re-run safely.
set -euo pipefail

NPM_PKG="@ostwal/aws-mcp-turbo"
REPO="aayushostwal/aws-mcp-turbo"
WORKFLOW="release.yml"
NPM_CLI="npm@11.20.0"

version="${1:-}"
shift || true
assume_yes=false
setup_only=false
for arg in "$@"; do
  case "$arg" in
    --yes) assume_yes=true ;;
    --setup-only) setup_only=true ;;
    *) echo "unknown option: $arg" >&2; exit 2 ;;
  esac
done

if [[ ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "usage: $0 X.Y.Z [--yes] [--setup-only]" >&2
  exit 2
fi
tag="v$version"

step() { printf '\n==> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

cd "$(git rev-parse --show-toplevel)"

step "Tools"
for tool in git node npx gh; do
  command -v "$tool" >/dev/null || die "$tool is not installed"
done
node_major="$(node -p 'process.versions.node.split(".")[0]')"
(( node_major >= 22 )) || die "Node 22 or newer is required (found $(node --version)); run: nvm install 22 && nvm use 22"
gh auth status >/dev/null 2>&1 || die "gh is not logged in; run: gh auth login"

step "Repository state"
[[ "$(git branch --show-current)" == "main" ]] || die "switch to main first: git switch main"
[[ -z "$(git status --porcelain)" ]] || die "working tree has uncommitted changes"
git fetch --quiet origin main --tags
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || die "main is not up to date: git pull --ff-only"
pkg_version="$(node -p "require('./npm/package.json').version")"
[[ "$pkg_version" == "$version" ]] || die "npm/package.json is $pkg_version, expected $version"
[[ -f "docs/releases/$tag.md" ]] || die "missing docs/releases/$tag.md"
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null || git ls-remote --exit-code --tags origin "$tag" >/dev/null 2>&1; then
  die "tag $tag already exists; never move a published tag"
fi
node scripts/check-release.mjs "$tag"

step "npm"
published="$(npm view "$NPM_PKG" version 2>/dev/null || true)"
echo "latest on npm: ${published:-none}"
if npm view "$NPM_PKG@$version" version >/dev/null 2>&1; then
  die "$NPM_PKG@$version is already on npm"
fi

step "npm trusted publishing"
if npx -y "$NPM_CLI" trust list "$NPM_PKG" 2>/dev/null | grep -q "$REPO"; then
  echo "trusted publisher for $REPO is already configured"
else
  echo "not configured (or not logged in). Logging in and configuring now; npm will ask for 2FA."
  npx -y "$NPM_CLI" login
  npx -y "$NPM_CLI" trust github "$NPM_PKG" --file "$WORKFLOW" --repo "$REPO" --allow-publish
  npx -y "$NPM_CLI" trust list "$NPM_PKG"
  echo
  echo "Check that the list above shows $REPO, $WORKFLOW and permission to publish."
fi
if $setup_only; then
  echo "setup complete; not tagging (--setup-only)"
  exit 0
fi

step "Ready to release $tag"
echo "commit:  $(git log -1 --format='%h %s')"
echo "action:  create tag $tag and push it; this starts the public release and npm publish"
if ! $assume_yes; then
  read -r -p "Type the version ($version) to continue: " answer
  [[ "$answer" == "$version" ]] || die "aborted"
fi

step "Tag and push"
git tag -a "$tag" -m "aws-mcp-turbo $tag"
git push origin "$tag"

step "Release workflow"
run_id=""
for _ in $(seq 1 30); do
  run_id="$(gh run list --workflow "$WORKFLOW" --branch "$tag" --limit 1 --json databaseId -q '.[0].databaseId' 2>/dev/null || true)"
  [[ -n "$run_id" ]] && break
  sleep 2
done
[[ -n "$run_id" ]] || die "workflow run did not start; check https://github.com/$REPO/actions"
gh run watch "$run_id" --exit-status || {
  echo >&2
  echo "workflow failed. If the GitHub release exists and only the npm job failed, fix the cause and run:" >&2
  echo "  gh run rerun $run_id --failed" >&2
  exit 1
}

step "Verify"
npm view "$NPM_PKG" version dist-tags
gh release view "$tag" --json url -q .url
npx -y "$NPM_PKG@latest" --version
echo
echo "released $tag"
