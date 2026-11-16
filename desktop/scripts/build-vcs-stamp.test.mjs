import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const buildScript = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'scripts', 'build.ps1');

// Loads only Assert-VcsRevisionStamped from build.ps1, with go stubbed to print lines and exit.
function runStampCheck(lines, exitCode = 0) {
  const quoted = lines.map((line) => `'${line.replaceAll("'", "''")}'`).join(', ');
  const command = [
    `$ast = [System.Management.Automation.Language.Parser]::ParseFile('${buildScript.replaceAll("'", "''")}', [ref]$null, [ref]$null)`,
    "$fn = $ast.Find({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq 'Assert-VcsRevisionStamped' }, $true)",
    '. ([scriptblock]::Create($fn.Extent.Text))',
    `function go { $global:LASTEXITCODE = ${exitCode}; @(${quoted}) }`,
    `try { $s = Assert-VcsRevisionStamped -Path 'C:\\bin\\zv.exe'; Write-Output "ok|$($s.Revision)|$($s.Modified)" }` +
      ' catch { Write-Output "throw|$($_.Exception.Message)" }',
  ].join('; ');
  const result = spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', command], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  return result.stdout.trim();
}

const header = ['C:\\bin\\zv.exe: go1.26.10', '\tpath\tgithub.com/rechedev9/cliphub/cmd/zv', '\tbuild\t-ldflags="-s -w"'];
const revision = '5a1b11810d8740602ee0ef4a720b09b50c9ba465';

test('a release binary must carry the vcs.revision the library retry compares', {
  skip: process.platform !== 'win32',
}, () => {
  const cases = [
    { name: 'clean tree', lines: [...header, '\tbuild\tvcs=git', `\tbuild\tvcs.revision=${revision}`, '\tbuild\tvcs.modified=false'], want: `ok|${revision}|False` },
    { name: 'modified tree', lines: [...header, `\tbuild\tvcs.revision=${revision}`, '\tbuild\tvcs.modified=true'], want: `ok|${revision}|True` },
    { name: 'no VCS stamp', lines: header, want: /^throw\|zv\.exe carries no vcs\.revision/ },
    { name: 'empty revision', lines: [...header, '\tbuild\tvcs.revision='], want: /^throw\|zv\.exe carries no vcs\.revision/ },
    { name: 'go version fails', lines: [], exitCode: 1, want: /^throw\|go version -m failed/ },
  ];
  for (const spec of cases) {
    const got = runStampCheck(spec.lines, spec.exitCode);
    if (typeof spec.want === 'string') assert.equal(got, spec.want, spec.name);
    else assert.match(got, spec.want, spec.name);
  }
});

test('build.ps1 checks the stamp of each binary before publishing', () => {
  const source = readFileSync(buildScript, 'utf8');
  const goBuild = source.search(/^\s+& go build\b/m);
  const check = source.search(/^\s+\$stamp = Assert-VcsRevisionStamped -Path \$out\s*$/m);
  const publish = source.search(/^\s+Publish-BuildArtifacts\b/m);
  assert.notEqual(check, -1, 'build.ps1 must call Assert-VcsRevisionStamped on each artifact');
  assert.ok(goBuild < check && check < publish, 'the stamp check runs after go build and before publication');
});
