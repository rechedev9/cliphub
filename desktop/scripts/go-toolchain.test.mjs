import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const repo = join(dirname(fileURLToPath(import.meta.url)), '..', '..');

function readRepo(relPath) {
  return readFileSync(join(repo, relPath), 'utf8');
}

function goModPatchVersion(source) {
  const match = /^go (\d+\.\d+\.\d+)$/m.exec(source);
  assert.ok(match, 'go.mod must pin a patch-level Go version');
  return match[1];
}

test('Windows installer rebuild pin agrees with go.mod 1.26.10', () => {
  const goMod = readRepo('go.mod');
  const version = goModPatchVersion(goMod);
  const pin = JSON.parse(readRepo('scripts/go-windows.json'));
  const installer = readRepo('scripts/install-go-windows.ps1');
  const cases = [
    {
      path: 'go.mod',
      body: goMod,
      must: [`go ${version}`],
      mustNot: ['1.26.5'],
    },
    {
      path: 'scripts/go-windows.json',
      body: JSON.stringify(pin),
      must: [
        version,
        `go${version}.windows-amd64.msi`,
        `https://go.dev/dl/go${version}.windows-amd64.msi`,
        `go${version}.windows-amd64.zip`,
        `https://go.dev/dl/go${version}.windows-amd64.zip`,
      ],
      mustNot: ['1.26.5'],
    },
    {
      path: 'scripts/install-go-windows.ps1',
      body: installer,
      must: [
        'Install-PinnedWindowsGo',
        'Invoke-WebRequest',
        'archiveSha256',
        'Expand-Archive',
        'GOTOOLCHAIN',
        'go.dev/dl/',
      ],
      mustNot: ['1.26.5'],
    },
    {
      path: '.github/workflows/desktop-release.yml',
      body: readRepo('.github/workflows/desktop-release.yml'),
      mustNot: ['1.26.5'],
    },
    {
      path: 'scripts/build.ps1',
      body: readRepo('scripts/build.ps1'),
      mustNot: ['1.26.5'],
    },
    {
      path: 'desktop/scripts/build-environment.mjs',
      body: readRepo('desktop/scripts/build-environment.mjs'),
      mustNot: ['1.26.5'],
    },
  ];

  assert.equal(version, '1.26.10');
  assert.equal(pin.version, version);
  assert.equal(pin.filename, `go${version}.windows-amd64.msi`);
  assert.equal(pin.url, `https://go.dev/dl/go${version}.windows-amd64.msi`);
  assert.equal(pin.archiveFilename, `go${version}.windows-amd64.zip`);
  assert.equal(pin.archiveUrl, `https://go.dev/dl/go${version}.windows-amd64.zip`);
  assert.match(pin.sha256, /^[a-f0-9]{64}$/);
  assert.match(pin.archiveSha256, /^[a-f0-9]{64}$/);
  assert.equal(pin.sha256, 'baf134644006fc229c7a119ca6248eaccab8472f10d718bdde71ad1de2fb0109');
  assert.equal(pin.archiveSha256, '7ecd2d7260e82c59bcbd7f1077a07cee2dea607295b858df67195a1e4e09629b');

  for (const spec of cases) {
    for (const needle of spec.must ?? []) {
      assert.equal(spec.body.includes(needle), true, `${spec.path} missing ${needle}`);
    }
    for (const needle of spec.mustNot) {
      assert.equal(spec.body.includes(needle), false, `${spec.path} still mentions ${needle}`);
    }
  }
});

test('Windows Go bootstrap probes the installed binary without auto toolchain substitution', {
  skip: process.platform !== 'win32',
}, () => {
  const installer = join(repo, 'scripts', 'install-go-windows.ps1');
  const cases = [
    { name: 'unset toolchain', ambient: null },
    { name: 'automatic toolchain', ambient: 'auto' },
    { name: 'explicit newer toolchain', ambient: 'go1.26.6' },
  ];

  for (const spec of cases) {
    const setAmbient = spec.ambient === null
      ? '[Environment]::SetEnvironmentVariable("GOTOOLCHAIN", $null, "Process")'
      : `$env:GOTOOLCHAIN = '${spec.ambient}'`;
    const command = [
      'function go {',
      "  if ($env:GOTOOLCHAIN -eq 'local') { 'go version go1.26.3 windows/amd64' }",
      "  else { 'go version go1.26.6 windows/amd64' }",
      '}',
      `. '${installer.replaceAll("'", "''")}'`,
      setAmbient,
      '$version = Get-LocalGoVersion',
      '$restored = if (Test-Path Env:GOTOOLCHAIN) { $env:GOTOOLCHAIN } else { "<unset>" }',
      'Write-Output "$version|$restored"',
    ].join('; ');
    const result = spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', command], {
      encoding: 'utf8',
    });

    assert.equal(result.status, 0, `${spec.name}: ${result.stderr}`);
    assert.equal(result.stdout.trim(), `1.26.3|${spec.ambient ?? '<unset>'}`, spec.name);
  }
});
