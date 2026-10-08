// Launch the compiled main process with a throwaway userData directory.
//
// main.ts takes app.requestSingleInstanceLock() and quits when another
// instance holds it. The lock is scoped per userData path, so a disposable
// profile (CLIPHUB_E2E_USER_DATA) never touches %APPDATA%\cliphub-studio.
//
/* eslint-disable @typescript-eslint/no-require-imports -- Electron loads this
   file as a CommonJS bootstrap before the app boots, so `require` is the only
   module system available: a .cjs file cannot use `import`, and the handover on
   the last line must happen synchronously *after* app.setPath, which a hoisted
   ESM import cannot express. Scoped to this file rather than relaxed in
   .oxlintrc.json, where the rule is correct for every other file. */
const path = require('node:path');
const { app } = require('electron');

const userData = process.env.CLIPHUB_E2E_USER_DATA;
if (!userData || !path.isAbsolute(userData)) {
  throw new Error('CLIPHUB_E2E_USER_DATA must be an absolute disposable profile path');
}
app.setPath('userData', userData);
delete process.env.CLIPHUB_E2E_USER_DATA;

require(path.join(__dirname, '..', 'dist', 'main.js'));
