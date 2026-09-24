// The renderer runs in its own Chromium process and never takes Studio's
// single-instance lock or starts the web server, capture workers or telemetry.
const overlayArgument = process.argv.indexOf('--cliphub-render-overlay');
if (overlayArgument >= 0) {
  void import('./overlay-renderer').then(({ runOverlayRenderer }) =>
    runOverlayRenderer(process.argv[overlayArgument + 1]),
  ).catch(async (error: unknown) => {
    process.stderr.write(`Overlay renderer: ${error instanceof Error ? error.message : String(error)}\n`);
    const { app } = await import('electron');
    app.exit(1);
  });
} else {
  // A failure while loading main (a missing module, a throwing constructor)
  // happens before main's own logging exists: leave a trace in studio.log and
  // tell the user instead of running on with no window.
  void import('./main').catch(async (error: unknown) => {
    const detail = error instanceof Error ? error.stack ?? `${error.name}: ${error.message}` : String(error);
    const { app, dialog } = await import('electron');
    try {
      const { appendFileSync } = await import('node:fs');
      const { join } = await import('node:path');
      appendFileSync(join(app.getPath('userData'), 'studio.log'), `[entry] main module failed to load: ${detail}\n`);
    } catch {
      // The dialog below is the remaining trace.
    }
    dialog.showErrorBox(
      'ClipHub Studio no pudo arrancar',
      `No se pudo cargar la aplicación. Reinstala ClipHub Studio; si el problema sigue, comparte studio.log.\n\n${detail}`,
    );
    app.exit(1);
  });
}
