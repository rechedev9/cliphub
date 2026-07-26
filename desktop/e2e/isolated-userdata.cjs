// E2e bootstrap: redirect userData before the app boots, then hand over to the
// real compiled main process.
//
// Why: main.ts takes app.requestSingleInstanceLock() and quits immediately when
// another instance holds it. The lock is scoped per userData path, so launching
// the suite against the default userData dies with a cryptic launch failure
// whenever a real FragForge Studio is open. Pointing userData at a dedicated
// stable tmp directory gives the suite its own lock (and warm tool
// provisioning cache) without touching the user's profile. We only set
// userData — the app name and every bundled resource stay untouched, because
// dist/main.js resolves resources from its own __dirname.
//
/* eslint-disable @typescript-eslint/no-require-imports -- Electron loads this
   file as a CommonJS bootstrap before the app boots, so `require` is the only
   module system available: a .cjs file cannot use `import`, and the handover on
   the last line must happen synchronously *after* app.setPath, which a hoisted
   ESM import cannot express. Scoped to this file rather than relaxed in
   .oxlintrc.json, where the rule is correct for every other file. */
const os = require('node:os');
const path = require('node:path');
const { app } = require('electron');

app.setPath('userData', path.join(os.tmpdir(), 'fragforge-studio-e2e-userdata'));

require(path.join(__dirname, '..', 'dist', 'main.js'));
