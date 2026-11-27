import { cpSync, existsSync, readdirSync, rmSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const standalone = join(dirname(fileURLToPath(import.meta.url)), '..', '.next', 'standalone');
const nested = join(standalone, 'web');

if (existsSync(join(nested, 'server.js'))) {
  for (const entry of readdirSync(nested)) {
    cpSync(join(nested, entry), join(standalone, entry), { recursive: true, force: true });
  }
  rmSync(nested, { recursive: true, force: true });
}

if (!existsSync(join(standalone, 'server.js'))) {
  throw new Error(`standalone Next server not found at ${join(standalone, 'server.js')}`);
}
