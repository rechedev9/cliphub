import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const script = readFileSync(new URL('../../scripts/measure-desktop-efficiency.ps1', import.meta.url), 'utf8');

test('Windows sampler preserves fractional CPU deltas instead of selecting integer Math.Max', {
  skip: process.platform !== 'win32',
}, () => {
  const calculation = script.split(/\r?\n/).find((line) => line.trim().startsWith('$cpuDelta ='));
  assert.ok(calculation);
  const code = `$previousCpu = @{ 42 = 1.0 }; $pidValue = 42; $cpuNow = 1.125; ${calculation}; $cpuDelta.ToString([Globalization.CultureInfo]::InvariantCulture)`;
  const output = execFileSync('powershell.exe', ['-NoProfile', '-Command', code], { encoding: 'utf8', timeout: 15_000 });
  assert.equal(output.trim(), '0.125');
  assert.match(script, /cpu_sampling_version = 2/);
  assert.match(script, /SignalReady -and \$sampleIndex -eq 0/);
});
