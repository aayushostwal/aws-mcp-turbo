import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';

const version = process.argv[2];
if (!/^v\d+\.\d+\.\d+$/.test(version || '')) throw new Error('Usage: node scripts/release.mjs vX.Y.Z');
const pkg = JSON.parse(await readFile('npm/package.json', 'utf8'));
if (`v${pkg.version}` !== version) throw new Error('Tag must match npm/package.json version');
await mkdir('dist', { recursive: true });
const checksums = [];
const hashes = {};
for (const os of ['darwin', 'linux']) {
  for (const arch of ['amd64', 'arm64']) {
    const name = `aws-mcp-turbo_${version}_${os}_${arch}`;
    execFileSync('go', ['build', '-trimpath', '-ldflags', `-s -w -X main.version=${version}`, '-o', `dist/${name}`, './cmd/aws-mcp-turbo'], {
      stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0', GOOS: os, GOARCH: arch },
    });
    const hash = createHash('sha256').update(await readFile(`dist/${name}`)).digest('hex');
    checksums.push(`${hash}  ${name}`);
    hashes[`${os}_${arch}`] = hash;
  }
}
await writeFile('dist/checksums.txt', checksums.join('\n') + '\n');
const formula = `class AwsMcpTurbo < Formula
  desc "Token-conscious AWS MCP server"
  homepage "https://github.com/aayushostwal/aws-mcp-turbo"
  version "${version.slice(1)}"
  license "MIT"

${['macos', 'linux'].map(platform => `  on_${platform} do
${['arm', 'intel'].map(cpu => {
  const os = platform === 'macos' ? 'darwin' : 'linux';
  const arch = cpu === 'arm' ? 'arm64' : 'amd64';
  return `    on_${cpu} do
      url "https://github.com/aayushostwal/aws-mcp-turbo/releases/download/${version}/aws-mcp-turbo_${version}_${os}_${arch}"
      sha256 "${hashes[`${os}_${arch}`]}"
    end`;
}).join('\n')}
  end`).join('\n\n')}

  def install
    bin.install Dir["aws-mcp-turbo_*"][0] => "aws-mcp-turbo"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/aws-mcp-turbo --version")
  end
end
`;
await writeFile('dist/aws-mcp-turbo.rb', formula);
console.log(`Built ${checksums.length} binaries, SHA-256 checksums, and Homebrew formula for ${version}`);
