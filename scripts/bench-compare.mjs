// Runs the same questions against aws-mcp-turbo and another MCP server over stdio
// and writes a transcript for scripts/token_metrics.py --compare. It makes real AWS
// calls with your credentials; every task is a read.
//
//   node scripts/bench-compare.mjs --turbo ./bin/aws-mcp-turbo \
//     --other "uvx mcp-proxy-for-aws-cli@latest https://aws-mcp.us-east-1.api.aws/mcp --metadata AWS_REGION=us-west-2" \
//     --var REGION_A=us-east-1 --var REGION_B=us-west-2 --var REGION_C=eu-west-1 --out bench/results.json
import { spawn } from 'node:child_process';
import { createInterface } from 'node:readline';
import { readFile, writeFile } from 'node:fs/promises';

const args = process.argv.slice(2);
const opt = (name, fallback) => { const i = args.indexOf(name); return i >= 0 ? args[i + 1] : fallback; };
const vars = Object.fromEntries(args.flatMap((a, i) => a === '--var' ? [args[i + 1].split(/=(.*)/s).slice(0, 2)] : []));
const turboCmd = (opt('--turbo', './bin/aws-mcp-turbo')).split(' ');
const otherCmd = opt('--other') ? opt('--other').split(' ') : null;
const out = opt('--out', 'bench/results.json');
const tasks = JSON.parse(await readFile(new URL('../bench/tasks.json', import.meta.url), 'utf8')).tasks;

function fill(value) {
  const text = JSON.stringify(value);
  const missing = [...text.matchAll(/\{\{(\w+)\}\}/g)].map(m => m[1]).filter(k => !(k in vars));
  if (missing.length) return { missing };
  return { value: JSON.parse(text.replace(/\{\{(\w+)\}\}/g, (_, k) => JSON.stringify(vars[k]).slice(1, -1))) };
}

async function connect(command) {
  const child = spawn(command[0], command.slice(1), { stdio: ['pipe', 'pipe', 'inherit'] });
  const pending = new Map();
  let id = 0;
  createInterface({ input: child.stdout }).on('line', line => {
    let m; try { m = JSON.parse(line); } catch { return; }
    const p = pending.get(m.id);
    if (p) { pending.delete(m.id); m.error ? p.reject(new Error(JSON.stringify(m.error))) : p.resolve(m.result); }
  });
  const call = (method, params) => new Promise((resolve, reject) => {
    const n = ++id; pending.set(n, { resolve, reject });
    child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id: n, method, params }) + '\n');
  });
  await call('initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'bench', version: '1' } });
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
  return { call, close: () => child.kill() };
}

async function run(label, command, key) {
  const server = await connect(command);
  const tools = (await server.call('tools/list', {})).tools;
  const result = { label, tools, tasks: {} };
  for (const task of tasks) {
    const filled = fill(task[key]);
    if (filled.missing) { result.tasks[task.id] = { skipped: `needs ${filled.missing.join(', ')}` }; continue; }
    const calls = [];
    for (const c of filled.value) {
      const start = performance.now();
      let response, error = false;
      try {
        const r = await server.call('tools/call', { name: c.tool, arguments: c.arguments });
        response = (r.content ?? []).map(x => x.text ?? '').join('\n'); error = !!r.isError;
      } catch (e) { response = String(e.message); error = true; }
      calls.push({ tool: c.tool, arguments: c.arguments, response, error, ms: Math.round(performance.now() - start) });
    }
    result.tasks[task.id] = { question: task.question, calls };
    console.error(`${label} ${task.id}: ${calls.map(c => `${c.ms}ms${c.error ? ' ERROR' : ''}`).join(', ')}`);
  }
  server.close();
  return result;
}

const results = [await run('turbo', turboCmd, 'turbo')];
if (otherCmd) results.push(await run('other', otherCmd, 'official'));
await writeFile(out, JSON.stringify(results, null, 1));
console.error(`wrote ${out}`);
