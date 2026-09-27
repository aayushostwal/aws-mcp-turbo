import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createInterface } from 'node:readline';
import { once } from 'node:events';

const npmSpec = process.argv[2] === '--npm' ? process.argv[3] : null;
if (process.argv[2] && !npmSpec) throw new Error('Usage: node scripts/stdio-smoke.mjs [--npm package-spec]');
const child = spawn(npmSpec ? 'npm' : process.env.AWS_MCP_TURBO_BINARY || './bin/aws-mcp-turbo',
  npmSpec ? ['exec', '--yes', '--package', npmSpec, '--', 'aws-mcp-turbo'] : [], {
  stdio: ['pipe', 'pipe', 'pipe'],
  env: { ...process.env, AWS_EC2_METADATA_DISABLED: 'true' },
});
const closed = once(child, 'exit');
const pending = new Map();
let stderr = '';
child.stderr.on('data', chunk => { stderr += chunk; });
child.on('error', error => { for (const { reject } of pending.values()) reject(error); });
child.on('exit', code => { for (const { reject } of pending.values()) reject(new Error(`Server exited ${code}: ${stderr}`)); });
createInterface({ input: child.stdout }).on('line', line => {
  let message;
  try { message = JSON.parse(line); } catch { throw new Error(`Non-JSON stdout: ${line}`); }
  if (message.id !== undefined) {
    const request = pending.get(message.id);
    if (request) { pending.delete(message.id); message.error ? request.reject(new Error(JSON.stringify(message.error))) : request.resolve(message.result); }
  }
});
let id = 0;
function call(method, params) {
  const requestId = ++id;
  return new Promise((resolve, reject) => {
    pending.set(requestId, { resolve, reject });
    child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id: requestId, method, params }) + '\n');
  });
}
const timer = setTimeout(() => { child.kill('SIGKILL'); process.exitCode = 1; }, 15000);
try {
  const init = await call('initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'smoke', version: '1' } });
  assert.equal(init.serverInfo.name, 'aws-mcp-turbo');
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
  const tools = await call('tools/list', {});
  assert.deepEqual(tools.tools.map(t => t.name).sort(), ['aws_diagnose', 'aws_discover', 'aws_mutate', 'aws_query']);
  const discovery = await call('tools/call', { name: 'aws_discover', arguments: { search: 'ec2.DescribeInstances' } });
  assert.ok(discovery.content[0].text.includes('InstanceIds'));
  const mutation = await call('tools/call', { name: 'aws_mutate', arguments: { action: 'ec2.StopInstances', intent: 'smoke', execute: true } });
  assert.equal(mutation.isError, true);
  assert.match(mutation.content[0].text, /disabled/);
  console.log('stdio initialize, tools/list, discover, and mutation denial passed');
} finally {
  child.stdin.end();
  try {
    const [code] = await closed;
    assert.equal(code, 0, `Server must shut down cleanly: ${stderr}`);
  } finally {
    clearTimeout(timer);
  }
}
