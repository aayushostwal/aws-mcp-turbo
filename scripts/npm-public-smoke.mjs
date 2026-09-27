// Networked release gate. Ordinary PR CI uses the offline npm-smoke.mjs instead.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { retryNpmInstall, validateRelease, waitForVersion } from './release-utils.mjs';

const pkg = JSON.parse(await readFile('npm/package.json', 'utf8'));
validateRelease(`v${pkg.version}`, pkg);
const local = process.argv[2] === '--local';
if (process.argv[2] && !local) throw new Error('Usage: node scripts/npm-public-smoke.mjs [--local]');
if (!local) await waitForVersion(pkg.version);
const directory = await mkdtemp(join(tmpdir(), 'aws-mcp-turbo-public-smoke-'));
const env = { ...process.env, NPM_CONFIG_USERCONFIG: '/dev/null', NPM_CONFIG_CACHE: join(directory, 'npm-cache'),
  NPM_CONFIG_REGISTRY: 'https://registry.npmjs.org', XDG_CACHE_HOME: join(directory, 'binary-cache') };
for (const key of ['AWS_MCP_TURBO_BINARY', 'NODE_AUTH_TOKEN', 'NPM_TOKEN',
  'ACTIONS_ID_TOKEN_REQUEST_URL', 'ACTIONS_ID_TOKEN_REQUEST_TOKEN']) delete env[key];
try {
  let spec = `${pkg.name}@${pkg.version}`;
  if (local) {
    const [packed] = JSON.parse(execFileSync('npm', ['pack', '--json', '--ignore-scripts', '--pack-destination', directory], {
      cwd: 'npm', env, encoding: 'utf8', timeout: 60000,
    }));
    spec = join(directory, packed.filename);
  }
  const output = (await retryNpmInstall(() => execFileSync('npm', ['exec', '--yes', '--package', spec, '--', 'aws-mcp-turbo', '--version'], {
    cwd: directory, env, encoding: 'utf8', timeout: 150000,
  }))).trim();
  assert.equal(output, `v${pkg.version}`);
  execFileSync(process.execPath, [fileURLToPath(new URL('./stdio-smoke.mjs', import.meta.url)), '--npm', spec], {
    cwd: directory, env: { ...env, NPM_CONFIG_OFFLINE: 'true' }, stdio: 'inherit', timeout: 30000,
  });
  console.log(`${local ? 'Packed' : 'Public'} npm install, verified binary, cached MCP startup, and discovery passed (${output})`);
} finally {
  await rm(directory, { recursive: true, force: true });
}
