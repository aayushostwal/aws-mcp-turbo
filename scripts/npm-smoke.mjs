// Exercise the published package layout and npx bin symlink without a registry
// publish or AWS credentials. The native binary must already be built.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const directory = await mkdtemp(join(tmpdir(), 'aws-mcp-turbo-npm-smoke-'));
try {
  const cache = join(directory, 'cache');
  const packed = JSON.parse(execFileSync('npm', ['pack', '--json', '--pack-destination', directory, '--cache', cache], {
    cwd: 'npm', encoding: 'utf8', timeout: 30000,
  }));
  const binary = resolve('bin/aws-mcp-turbo');
  const expected = execFileSync(binary, ['--version'], { encoding: 'utf8' }).trim();
  const output = execFileSync('npm', ['exec', '--offline', '--yes', '--cache', cache,
    '--package', join(directory, packed[0].filename), '--', 'aws-mcp-turbo', '--version'], {
    cwd: directory, env: { ...process.env, AWS_MCP_TURBO_BINARY: binary },
    encoding: 'utf8', timeout: 30000,
  }).trim();
  assert.equal(output, expected, 'npx must launch the binary, not silently exit');
  execFileSync(process.execPath, ['scripts/stdio-smoke.mjs', '--npm', join(directory, packed[0].filename)], {
    env: { ...process.env, AWS_MCP_TURBO_BINARY: binary, NPM_CONFIG_CACHE: cache, NPM_CONFIG_OFFLINE: 'true' },
    stdio: 'inherit', timeout: 30000,
  });
  console.log(`Packed npm/npx executable smoke passed (${output})`);
} finally {
  await rm(directory, { recursive: true, force: true });
}
