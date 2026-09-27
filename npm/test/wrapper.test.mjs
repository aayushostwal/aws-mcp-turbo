import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { assetName, checksumFor, binaryPath } from '../bin/aws-mcp-turbo.mjs';

test('maps supported release assets and rejects path injection', () => {
  assert.equal(assetName('0.1.0', 'linux', 'x64'), 'aws-mcp-turbo_v0.1.0_linux_amd64');
  assert.equal(assetName('0.1.0', 'darwin', 'arm64'), 'aws-mcp-turbo_v0.1.0_darwin_arm64');
  assert.throws(() => assetName('../escape', 'linux', 'x64'));
  assert.throws(() => assetName('0.1.0', 'win32', 'x64'));
});

test('requires a unique exact SHA-256 entry', () => {
  const hash = 'a'.repeat(64);
  assert.equal(checksumFor(`${hash}  binary\n`, 'binary'), hash);
  assert.throws(() => checksumFor(`${hash}  binary-other\n`, 'binary'));
  assert.throws(() => checksumFor(`${hash}  binary\n${hash}  binary`, 'binary'));
  assert.throws(() => checksumFor('deadbeef  binary', 'binary'));
});

test('local binary override must be absolute', async () => {
  const original = process.env.AWS_MCP_TURBO_BINARY;
  try {
    process.env.AWS_MCP_TURBO_BINARY = 'relative';
    await assert.rejects(binaryPath(), /absolute/);
    process.env.AWS_MCP_TURBO_BINARY = '/tmp/test-binary';
    assert.equal(await binaryPath(), '/tmp/test-binary');
  } finally {
    if (original === undefined) delete process.env.AWS_MCP_TURBO_BINARY;
    else process.env.AWS_MCP_TURBO_BINARY = original;
  }
});

test('npm-style executable symlink starts the native binary', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'aws-mcp-turbo-symlink-'));
  try {
    const link = join(directory, 'aws-mcp-turbo');
    await symlink(new URL('../bin/aws-mcp-turbo.mjs', import.meta.url), link);
    const result = spawnSync(process.execPath, [link, '--version'], {
      env: { ...process.env, AWS_MCP_TURBO_BINARY: process.execPath },
      encoding: 'utf8', timeout: 15000,
    });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout.trim(), process.version);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
