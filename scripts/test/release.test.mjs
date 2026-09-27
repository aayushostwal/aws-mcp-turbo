import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { isNewer, packageName, publicationDecision, registryVersion, retryNpmInstall, validateRelease, waitForVersion } from '../release-utils.mjs';

const pkg = { name: packageName, version: '0.1.1' };
const packed = { ...pkg, integrity: 'sha512-example' };
const published = { ...pkg, dist: { integrity: packed.integrity } };

test('release requires the exact stable version and expected package name', () => {
  assert.equal(validateRelease('v0.1.1', pkg), '0.1.1');
  for (const tag of [undefined, '0.1.1', 'v0.1.0', 'v0.1.1-beta', '../0.1.1']) {
    assert.throws(() => validateRelease(tag, pkg));
  }
  for (const version of ['01.1.1', '0.1.1-beta', '0.1.1+build', '../../bad']) {
    assert.throws(() => validateRelease(`v${version}`, { ...pkg, version }));
  }
  assert.throws(() => validateRelease('v0.1.1', { ...pkg, name: '@other/pkg' }));
});

test('publishes only absent versions; skips byte-identical reruns', () => {
  assert.equal(publicationDecision(404, null, packed), 'publish');
  assert.equal(publicationDecision(200, published, packed), 'skip');
});

test('registry/auth errors and conflicting existing packages fail closed', () => {
  for (const status of [401, 403, 429, 500, 503]) {
    assert.throws(() => publicationDecision(status, null, packed), /refusing/);
  }
  for (const remote of [{ ...published, name: '@other/pkg' }, { ...published, version: '0.1.0' },
    { ...published, dist: { integrity: 'sha512-different' } }, { ...pkg }]) {
    assert.throws(() => publicationDecision(200, remote, packed), /never overwrite/);
  }
});

test('semver comparison prevents moving latest backwards', () => {
  for (const version of ['0.1.2', '0.2.0', '1.0.0', '0.10.0']) assert.equal(isNewer(version, '0.1.1'), true);
  for (const version of ['0.1.0', '0.0.9', '0.1.1']) assert.equal(isNewer(version, '0.1.1'), false);
  assert.equal(isNewer('0.9.0', '0.10.0'), false);
  assert.throws(() => isNewer('0.1.1', 'beta'));
});

test('public registry lookup uses the exact package, version and timeout', async () => {
  const result = await registryVersion('0.1.1', async (url, options) => {
    assert.equal(url, 'https://registry.npmjs.org/%40ostwal%2Faws-mcp-turbo/0.1.1');
    assert.ok(options.signal instanceof AbortSignal);
    return { status: 200, json: async () => published };
  });
  assert.deepEqual(result, { status: 200, data: published });
  await assert.rejects(registryVersion('../bad'), /Invalid registry version/);
});

test('registry visibility retries propagation delays and transient errors', async () => {
  const statuses = [404, 429, 503, 200];
  let sleeps = 0;
  assert.deepEqual(await waitForVersion('0.1.1', {
    lookup: async () => ({ status: statuses.shift(), data: published }),
    sleep: async ms => { assert.equal(ms, 10000); sleeps++; },
  }), published);
  assert.equal(sleeps, 3);
});

test('registry visibility has bounded retries and rejects wrong versions/auth errors', async () => {
  let calls = 0;
  await assert.rejects(waitForVersion('0.1.1', { attempts: 3,
    lookup: async () => { calls++; return { status: 404 }; }, sleep: async () => {},
  }), /bounded retries/);
  assert.equal(calls, 3);
  await assert.rejects(waitForVersion('0.1.1', { lookup: async () => ({ status: 403 }) }), /403/);
  await assert.rejects(waitForVersion('0.1.1', {
    lookup: async () => ({ status: 200, data: { ...pkg, version: '0.1.0' } }),
  }), /wrong package/);
});

test('workflow keeps tag-only publishing, binary-first ordering, and tokenless auth', async () => {
  const workflow = await readFile(new URL('../../.github/workflows/release.yml', import.meta.url), 'utf8');
  assert.match(workflow, /tags: \['v\*\.\*\.\*'\]/);
  assert.doesNotMatch(workflow, /pull_request:|workflow_dispatch:|secrets\.NPM|--clobber|--draft/);
  // setup-node's registry-url writes an empty NODE_AUTH_TOKEN placeholder,
  // which suppresses npm's tokenless trusted-publishing authentication.
  assert.doesNotMatch(workflow, /registry-url:/);
  assert.match(workflow, /needs: release/);
  assert.match(workflow, /id-token: write/);
  assert.match(workflow, /git merge-base --is-ancestor/);
  assert.match(workflow, /cancel-in-progress: false/);
  assert.ok(workflow.indexOf('npm-public-smoke.mjs --local') < workflow.indexOf('publish-npm.mjs'));
  assert.ok(workflow.lastIndexOf('npm-public-smoke.mjs') > workflow.indexOf('publish-npm.mjs'));
});

test('npm index propagation retries are bounded and do not hide binary failures', async () => {
  let calls = 0;
  const missing = Object.assign(new Error('missing'), { stderr: 'npm error code E404' });
  assert.equal(await retryNpmInstall(() => {
    if (++calls < 3) throw missing;
    return 'v0.1.1';
  }, { sleep: async () => {} }), 'v0.1.1');
  assert.equal(calls, 3);
  calls = 0;
  await assert.rejects(retryNpmInstall(() => { calls++; throw missing; }, {
    attempts: 2, sleep: async () => {},
  }), /missing/);
  assert.equal(calls, 2);
  calls = 0;
  await assert.rejects(retryNpmInstall(() => {
    calls++;
    throw Object.assign(new Error('checksum failure'), { stderr: 'Binary SHA-256 verification failed' });
  }), /checksum failure/);
  assert.equal(calls, 1);
});
