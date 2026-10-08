// Fails a Studio release whose bundled HLAE is not advancedfx's latest release.
// Studio also installs the latest HLAE at boot, but the bundled archive is what
// a fresh or offline install captures with, so it must never be stale.
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ADVANCEDFX_LATEST_RELEASE_URL, parseLatestHLAERelease } from '../src/hlae-latest.ts';

const desktop = join(dirname(fileURLToPath(import.meta.url)), '..');
const pinned = JSON.parse(readFileSync(join(desktop, 'src', 'hlae-tool.json'), 'utf8'));

const headers = { accept: 'application/vnd.github+json', 'user-agent': 'ClipHub-release' };
if (process.env.GITHUB_TOKEN) headers.authorization = `Bearer ${process.env.GITHUB_TOKEN}`;

async function main() {
  const response = await fetch(ADVANCEDFX_LATEST_RELEASE_URL, { headers });
  if (!response.ok) return `cannot read ${ADVANCEDFX_LATEST_RELEASE_URL}: HTTP ${response.status}`;
  const latest = parseLatestHLAERelease(await response.json());
  if (!latest) return 'the latest advancedfx release has no verifiable portable zip';
  if (latest.version !== pinned.version || latest.sha256 !== pinned.sha256) {
    return `bundled HLAE ${pinned.version} is not the latest release ${latest.version}. `
      + 'Pin it in desktop/src/hlae-tool.json and hlae-tool.ts (see docs/incidents.md, HLAE pin and CS2 updates), prove it with a real capture, then release.';
  }
  console.log(`[hlae] bundled HLAE ${pinned.version} is the latest advancedfx release`);
  return null;
}

// exitCode, not exit(): a forced exit with fetch sockets open aborts libuv on Windows.
const failure = await main();
if (failure) {
  console.error(`[hlae] ${failure}`);
  process.exitCode = 1;
}
