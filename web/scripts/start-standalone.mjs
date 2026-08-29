import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { pathToFileURL, fileURLToPath } from 'node:url';

const webRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const standalone = join(webRoot, '.next', 'standalone');
const server = join(standalone, 'server.js');
if (!existsSync(server)) {
  throw new Error(`standalone Next server not found at ${server}; run pnpm run build first`);
}

// Next intentionally leaves public/ and .next/static/ outside output=standalone.
// Populate the runnable development copy immediately before launch; desktop's
// assemble path continues to stage its own immutable distribution resources.
for (const [source, destination] of [
  [join(webRoot, 'public'), join(standalone, 'public')],
  [join(webRoot, '.next', 'static'), join(standalone, '.next', 'static')],
]) {
  rmSync(destination, { recursive: true, force: true });
  mkdirSync(dirname(destination), { recursive: true });
  cpSync(source, destination, { recursive: true });
}

process.chdir(standalone);
await import(pathToFileURL(server).href);
