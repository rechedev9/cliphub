import { copyFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const unrarPackage = dirname(createRequire(import.meta.url).resolve('node-unrar-js/package.json'));
// Same-origin fetch for browser unrar; copied so it tracks the npm package version.
copyFileSync(join(unrarPackage, 'esm/js/unrar.wasm'), join(here, 'public/unrar.wasm'));

const securityHeaders = [
  {
    key: 'Content-Security-Policy',
    // Next's runtime and CSS-in-JS output contain inline bootstrap/style data.
    // Keep this compatible baseline until those assets are nonce/hash based.
    value: "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; connect-src 'self'; worker-src 'self' blob:",
  },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'Permissions-Policy', value: 'camera=(), geolocation=(), microphone=(), payment=()' },
];

// A team document opens in the app's own iframe, so it may be framed by the app (and only by it), always sandboxed.
// Equal to TeamDocumentPolicy in internal/httpapi/library_team_documents.go; a later rule overrides the same keys.
const teamDocumentHeaders = [
  {
    key: 'Content-Security-Policy',
    value: "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox; default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' https:; style-src 'unsafe-inline' https:; font-src https: data:; img-src https: data: blob:; media-src https: data: blob:; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'self'",
  },
  { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
  { key: 'Referrer-Policy', value: 'no-referrer' },
];

/** @type {import('next').NextConfig} */
export default {
  // Do not recreate web/AGENTS.md and web/CLAUDE.md on `next dev`.
  agentRules: false,
  poweredByHeader: false,
  // Emit the self-contained server bundle assembled into the desktop installer.
  output: 'standalone',
  // Pin the file-tracing root to the repo root, where the workspace hoists node_modules. Without it,
  // Next walks up and can pick a stray parent package.json, which nests the standalone output
  // under the build machine's absolute path. The server lands in standalone/web/server.js;
  // scripts/flatten-standalone.mjs moves it to standalone/server.js, the layout the desktop
  // installer and the ClipMind image expect.
  outputFileTracingRoot: join(here, '..'),
  // No next/image optimization: /_next/image returns 404 and Sharp never loads,
  // so the standalone trace leaves out Sharp and its ~19 MB native runtime.
  images: { unoptimized: true },
  outputFileTracingExcludes: { 'next-server': ['**/node_modules/sharp/**', '**/node_modules/@img/**'] },
  webpack(config) {
    // The algorithms guide is Markdown bundled as text, so no runtime file read.
    config.module.rules.push({ test: /\.md$/, type: 'asset/source' });
    return config;
  },
  async headers() {
    return [
      { source: '/:path*', headers: securityHeaders },
      { source: '/api/library/documents/:id/content', headers: teamDocumentHeaders },
    ];
  },
};
