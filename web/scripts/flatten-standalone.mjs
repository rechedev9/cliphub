import { cpSync, existsSync, readdirSync, rmSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const standalone = join(dirname(fileURLToPath(import.meta.url)), '..', '.next', 'standalone');
const nested = join(standalone, 'web');

function packageNames(modulesDirectory) {
  if (!existsSync(modulesDirectory)) return [];
  return readdirSync(modulesDirectory, { withFileTypes: true }).flatMap((entry) => {
    if (!entry.name.startsWith('@')) return [entry.name];
    return readdirSync(join(modulesDirectory, entry.name)).map((name) => `${entry.name}/${name}`);
  });
}

if (existsSync(join(nested, 'server.js'))) {
  const shadowed = packageNames(join(nested, 'node_modules')).filter((name) =>
    existsSync(join(standalone, 'node_modules', name)),
  );
  if (shadowed.length > 0) {
    throw new Error(
      `web/node_modules and the root node_modules both hold ${shadowed.join(', ')}; flattening would mix two versions. Align the versions across the workspace.`,
    );
  }
  for (const entry of readdirSync(nested)) {
    cpSync(join(nested, entry), join(standalone, entry), { recursive: true, force: true });
  }
  rmSync(nested, { recursive: true, force: true });
}

if (!existsSync(join(standalone, 'server.js'))) {
  throw new Error(`standalone Next server not found at ${join(standalone, 'server.js')}`);
}
