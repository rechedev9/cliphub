import { chmodSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const wasmDir = join(repoRoot, 'web', 'public', 'wasm');

try {
  const goroot = execFileSync('go', ['env', 'GOROOT'], { cwd: repoRoot, encoding: 'utf8' }).trim();
  mkdirSync(wasmDir, { recursive: true });
  execFileSync(
    'go',
    ['build', '-trimpath', '-ldflags=-s -w', '-o', join(wasmDir, 'zv-scan.wasm'), './cmd/zv-scan-wasm'],
    { cwd: repoRoot, stdio: 'inherit', env: { ...process.env, GOOS: 'js', GOARCH: 'wasm' } },
  );
  // The Go module cache is read-only, so copy the bytes rather than the file and its mode.
  const execPath = join(wasmDir, 'wasm_exec.js');
  if (existsSync(execPath)) chmodSync(execPath, 0o644);
  writeFileSync(execPath, readFileSync(join(goroot, 'lib', 'wasm', 'wasm_exec.js')));
} catch (error) {
  if (error.code === 'ENOENT') throw new Error('go is required to build the WebAssembly scanner; install Go and retry');
  throw error;
}
