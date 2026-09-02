import { spawn } from 'node:child_process';
import { join } from 'node:path';

const port = process.env.PORT ?? '3000';
const vite = join(process.cwd(), 'node_modules', 'vite', 'bin', 'vite.js');
const child = spawn(process.execPath, [vite, 'preview', '--host', '127.0.0.1', '--port', port, '--strictPort'], {
  stdio: 'inherit',
});
child.once('exit', (code) => process.exit(code ?? 1));
