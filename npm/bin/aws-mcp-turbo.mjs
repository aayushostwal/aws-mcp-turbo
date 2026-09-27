#!/usr/bin/env node
import { createHash, randomUUID } from 'node:crypto';
import { createReadStream, createWriteStream } from 'node:fs';
import { chmod, mkdir, readFile, realpath, rename, rm } from 'node:fs/promises';
import { homedir } from 'node:os';
import { isAbsolute, join } from 'node:path';
import { pipeline } from 'node:stream/promises';
import { Transform } from 'node:stream';
import { spawn } from 'node:child_process';
import { pathToFileURL } from 'node:url';

export function assetName(version, platform = process.platform, arch = process.arch) {
  if (!/^\d+\.\d+\.\d+$/.test(version)) throw new Error('Invalid release version');
  if (!['darwin', 'linux'].includes(platform) || !['arm64', 'x64'].includes(arch)) {
    throw new Error(`Unsupported platform: ${platform}/${arch}; build from source`);
  }
  return `aws-mcp-turbo_v${version}_${platform}_${arch === 'x64' ? 'amd64' : 'arm64'}`;
}

export function checksumFor(checksums, name) {
  const matches = checksums.split('\n').map(line => line.match(/^([a-f0-9]{64})  (\S+)$/)).filter(m => m?.[2] === name);
  if (matches.length !== 1) throw new Error(`Missing or ambiguous checksum for ${name}`);
  return matches[0][1];
}

async function digest(path) {
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return hash.digest('hex');
}

async function download(url, path, maxBytes) {
  const response = await fetch(url, { signal: AbortSignal.timeout(120_000) });
  if (!response.ok || !response.url.startsWith('https://')) throw new Error(`Download failed (${response.status}): ${url}`);
  let size = 0;
  const limit = new Transform({ transform(chunk, _, callback) {
    size += chunk.length;
    callback(size > maxBytes ? new Error('Release asset exceeds size limit') : null, chunk);
  }});
  await pipeline(response.body, limit, createWriteStream(path, { flags: 'wx', mode: 0o600 }));
}

export async function binaryPath() {
  if (process.env.AWS_MCP_TURBO_BINARY) {
    if (!isAbsolute(process.env.AWS_MCP_TURBO_BINARY)) throw new Error('AWS_MCP_TURBO_BINARY must be absolute');
    return process.env.AWS_MCP_TURBO_BINARY;
  }
  const pkg = JSON.parse(await readFile(new URL('../package.json', import.meta.url), 'utf8'));
  const name = assetName(pkg.version);
  const cache = join(process.env.XDG_CACHE_HOME || join(homedir(), '.cache'), 'aws-mcp-turbo', pkg.version);
  await mkdir(cache, { recursive: true, mode: 0o700 });
  const target = join(cache, name);
  const sums = join(cache, 'checksums.txt');
  try {
    const expected = checksumFor(await readFile(sums, 'utf8'), name);
    if (await digest(target) === expected) return target;
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }
  const base = `https://github.com/aayushostwal/aws-mcp-turbo/releases/download/v${pkg.version}`;
  const temporary = join(cache, `.download-${randomUUID()}`);
  const temporarySums = temporary + '.sha256';
  process.stderr.write(`aws-mcp-turbo: downloading verified v${pkg.version} binary\n`);
  try {
    await download(`${base}/checksums.txt`, temporarySums, 65536);
    const checksums = await readFile(temporarySums, 'utf8');
    const expected = checksumFor(checksums, name);
    await download(`${base}/${name}`, temporary, 200 * 1024 * 1024);
    if (await digest(temporary) !== expected) throw new Error('Binary SHA-256 verification failed');
    await chmod(temporary, 0o700);
    await rename(temporary, target);
    await rename(temporarySums, sums);
    return target;
  } finally {
    await rm(temporary, { force: true });
    await rm(temporarySums, { force: true });
  }
}

async function main() {
  const binary = await binaryPath();
  const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit' });
  for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal));
  child.on('error', error => { process.stderr.write(`${error.message}\n`); process.exitCode = 1; });
  child.on('exit', (code, signal) => { process.exitCode = code ?? (signal === 'SIGINT' ? 130 : 143); });
}

// npm/npx launch the bin through a symlink; Node resolves import.meta.url but
// preserves that symlink in argv[1]. Compare canonical paths to run the entrypoint.
if (process.argv[1] && import.meta.url === pathToFileURL(await realpath(process.argv[1])).href) {
  main().catch(error => { process.stderr.write(`aws-mcp-turbo: ${error.message}\n`); process.exitCode = 1; });
}
