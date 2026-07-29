// E2e bootstrap: redirect userData before the app boots, then hand over to the
// real compiled main process.
//
// Why: main.ts takes app.requestSingleInstanceLock() and quits immediately when
// another instance holds it. The lock is scoped per userData path, so every
// suite receives a unique disposable profile from scripts/e2e-profile.mjs.
// Any verified tool fixture is copied there before launch; suites never share
// mutable SQLite, cookies, ports, window state, or a live tool directory.
//
/* eslint-disable @typescript-eslint/no-require-imports -- Electron loads this
   file as a CommonJS bootstrap before the app boots, so `require` is the only
   module system available: a .cjs file cannot use `import`, and the handover on
   the last line must happen synchronously *after* app.setPath, which a hoisted
   ESM import cannot express. Scoped to this file rather than relaxed in
   .oxlintrc.json, where the rule is correct for every other file. */
const path = require('node:path');
const { app } = require('electron');

const userData = process.env.FRAGFORGE_E2E_USER_DATA;
if (!userData || !path.isAbsolute(userData)) {
  throw new Error('FRAGFORGE_E2E_USER_DATA must be an absolute disposable profile path');
}
app.setPath('userData', userData);
delete process.env.FRAGFORGE_E2E_USER_DATA;

require(path.join(__dirname, '..', 'dist', 'main.js'));
