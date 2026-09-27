import { execFileSync } from 'node:child_process';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { isNewer, publicationDecision, registryVersion, validateRelease } from './release-utils.mjs';

const pkg = JSON.parse(await readFile('npm/package.json', 'utf8'));
validateRelease(process.env.GITHUB_REF_NAME, pkg);
if (process.env.GITHUB_ACTIONS !== 'true' || !process.env.ACTIONS_ID_TOKEN_REQUEST_URL ||
    !process.env.ACTIONS_ID_TOKEN_REQUEST_TOKEN) throw new Error('Publishing requires GitHub Actions OIDC');
const directory = await mkdtemp(join(tmpdir(), 'aws-mcp-turbo-publish-'));
try {
  const [packed] = JSON.parse(execFileSync('npm', ['pack', '--json', '--ignore-scripts', '--pack-destination', directory], {
    cwd: 'npm', encoding: 'utf8', timeout: 60000,
  }));
  const remote = await registryVersion(pkg.version);
  if (publicationDecision(remote.status, remote.data, packed) === 'skip') {
    console.log(`Identical ${pkg.name}@${pkg.version} is already published; continuing verification`);
  } else {
    const latest = await registryVersion('latest');
    if (latest.status !== 200 || !isNewer(pkg.version, latest.data.version)) {
      throw new Error('New release must be newer than the public latest version; refusing to move latest backwards');
    }
    execFileSync('npm', ['publish', join(directory, packed.filename), '--access', 'public', '--provenance', '--ignore-scripts'], {
      stdio: 'inherit', timeout: 120000,
    });
  }
} finally {
  await rm(directory, { recursive: true, force: true });
}
