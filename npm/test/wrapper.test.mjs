import test from 'node:test';
import assert from 'node:assert/strict';
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
