import { readFile } from 'node:fs/promises';
import { validateRelease } from './release-utils.mjs';

const pkg = JSON.parse(await readFile('npm/package.json', 'utf8'));
const version = validateRelease(process.argv[2], pkg);
const notes = await readFile(`docs/releases/v${version}.md`, 'utf8');
if (!notes.trim()) throw new Error('Release notes must not be empty');
console.log(`Validated ${pkg.name}@${version} and release notes`);
