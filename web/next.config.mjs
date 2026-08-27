import { copyFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
// Same-origin fetch for browser unrar; copied so it tracks the npm package version.
copyFileSync(join(here, 'node_modules/node-unrar-js/esm/js/unrar.wasm'), join(here, 'public/unrar.wasm'));

const securityHeaders = [
  {
    key: 'Content-Security-Policy',
    // Next's runtime and CSS-in-JS output contain inline bootstrap/style data.
    // Keep this compatible baseline until those assets are nonce/hash based.
    value: "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; connect-src 'self' http://127.0.0.1:*; worker-src 'self' blob:",
  },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'Permissions-Policy', value: 'camera=(), geolocation=(), microphone=(), payment=()' },
];

/** @type {import('next').NextConfig} */
export default {
  poweredByHeader: false,
  // Emit the self-contained server bundle assembled into the desktop installer.
  output: 'standalone',
  // Pin the file-tracing root to this app. Without it, Next walks up and can pick
  // a parent package.json (e.g. a stray ~/package.json) as the workspace root,
  // which nests the standalone output under the build machine's absolute path
  // (…/standalone/<abs/path>/web/server.js) instead of a flat …/standalone/
  // server.js. Desktop packaging relies on the flat layout, so make it
  // deterministic regardless of where the repo is checked out.
  outputFileTracingRoot: here,
  async headers() {
    return [
      { source: '/generated/agent-sw.js', headers: [{ key: 'Service-Worker-Allowed', value: '/' }] },
      { source: '/:path*', headers: securityHeaders },
    ];
  },
};
