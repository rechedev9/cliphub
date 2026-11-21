// Launch Studio with a throwaway userData directory.
//
// main.ts takes app.requestSingleInstanceLock() and quits when another
// instance holds it. The lock is scoped per userData path, so a disposable
// profile (CLIPHUB_E2E_USER_DATA) never touches %APPDATA%\cliphub-studio.
//
// Electron's default_app sets the app path to the directory of the file it
// was given. This file lives in desktop/scripts, which has no package.json,
// so the Full Demo overlay renderer (electron <app path> --cliphub-render-overlay)
// would not find the app. Point the app path at desktop/ and load package.json
// "main" (dist/entry.js) so the version and the overlay branch are the app's.
//
const path = require('node:path');
const { app } = require('electron');

const userData = process.env.CLIPHUB_E2E_USER_DATA;
if (!userData || !path.isAbsolute(userData)) {
  throw new Error('CLIPHUB_E2E_USER_DATA must be an absolute disposable profile path');
}
const desktopRoot = path.join(__dirname, '..');
app.setPath('userData', userData);
app.setAppPath(desktopRoot);
app.setVersion(require(path.join(desktopRoot, 'package.json')).version);
delete process.env.CLIPHUB_E2E_USER_DATA;

require(path.join(desktopRoot, 'dist', 'entry.js'));
