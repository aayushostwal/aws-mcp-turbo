const stableVersion = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
export const packageName = '@ostwal/aws-mcp-turbo';

export function validateRelease(tag, pkg) {
  if (!stableVersion.test(pkg.version) || tag !== `v${pkg.version}`) {
    throw new Error('A stable vX.Y.Z tag must match npm/package.json version');
  }
  if (pkg.name !== packageName) throw new Error('Unexpected npm package name');
  return pkg.version;
}

export function publicationDecision(status, remote, packed) {
  if (status === 404) return 'publish';
  if (status !== 200) throw new Error(`Registry lookup failed (${status}); refusing to publish`);
  if (remote.name !== packed.name || remote.version !== packed.version ||
      !packed.integrity || remote.dist?.integrity !== packed.integrity) {
    throw new Error('Published version differs from the local tarball; never overwrite a version');
  }
  return 'skip';
}

export function isNewer(version, previous) {
  if (!stableVersion.test(version) || !stableVersion.test(previous)) {
    throw new Error('Expected stable semver versions');
  }
  const current = version.split('.').map(BigInt);
  const old = previous.split('.').map(BigInt);
  for (let i = 0; i < 3; i++) {
    if (current[i] !== old[i]) return current[i] > old[i];
  }
  return false;
}

export async function registryVersion(version, fetcher = fetch) {
  if (!stableVersion.test(version) && version !== 'latest') throw new Error('Invalid registry version');
  const response = await fetcher(`https://registry.npmjs.org/${encodeURIComponent(packageName)}/${version}`, {
    headers: { Accept: 'application/json' }, signal: AbortSignal.timeout(15000),
  });
  return { status: response.status, data: response.status === 200 ? await response.json() : null };
}

export async function waitForVersion(version, { attempts = 24, lookup = registryVersion,
  sleep = ms => new Promise(resolve => setTimeout(resolve, ms)) } = {}) {
  for (let attempt = 1; attempt <= attempts; attempt++) {
    const { status, data } = await lookup(version);
    if (status === 200) {
      if (data.name !== packageName || data.version !== version) throw new Error('Registry returned the wrong package/version');
      return data;
    }
    if (status !== 404 && status !== 429 && status < 500) throw new Error(`Registry lookup failed (${status})`);
    if (attempt < attempts) await sleep(10000);
  }
  throw new Error(`Published ${packageName}@${version} is not visible after bounded retries`);
}

// The per-version endpoint can become visible before npm's package index does.
export async function retryNpmInstall(run, { attempts = 12,
  sleep = ms => new Promise(resolve => setTimeout(resolve, ms)) } = {}) {
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try { return await run(); } catch (error) {
      if (!/npm error code (E404|ETARGET)\b/.test(String(error.stderr)) || attempt === attempts) throw error;
      await sleep(10000);
    }
  }
}
