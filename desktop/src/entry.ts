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
  void import('./main');
}
