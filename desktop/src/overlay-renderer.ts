import { app, BrowserWindow } from 'electron';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { parseOverlayRenderRequest } from './overlay-render-request';

/** Render an offline HTML/CSS overlay through Chromium with transparent pixels. */
export async function runOverlayRenderer(requestPath: string | undefined): Promise<void> {
  const trace = (stage: string): void => { if (process.env.ZV_OVERLAY_DEBUG === '1') process.stderr.write(`Overlay stage: ${stage}\n`); };
  if (!requestPath || !path.isAbsolute(requestPath)) throw new Error('An absolute overlay request path is required');
  if (fs.statSync(requestPath).size > 16 * 1024) throw new Error('Overlay request is too large');
  const request = parseOverlayRenderRequest(JSON.parse(fs.readFileSync(requestPath, 'utf8')));
  if (fs.statSync(request.html_path).size > 32 * 1024 * 1024) throw new Error('Overlay HTML is too large');
  const profile = path.join(path.dirname(requestPath), 'chromium-profile');
  fs.mkdirSync(profile, { recursive: true });
  app.setPath('userData', profile);
  app.commandLine.appendSwitch('force-device-scale-factor', '1');
  app.disableHardwareAcceleration();
  const timeout = setTimeout(() => {
    process.stderr.write('Overlay renderer timed out\n');
    app.exit(1);
  }, 25_000);
  await app.whenReady();
  trace('browser ready');
  const window = new BrowserWindow({
    width: request.width, height: request.height, useContentSize: true,
    show: false, frame: false, transparent: true, backgroundColor: '#00000000',
    webPreferences: {
      sandbox: true, contextIsolation: true, nodeIntegration: false,
      backgroundThrottling: false, offscreen: true, partition: `overlay-${process.pid}`,
    },
  });
  window.setContentSize(request.width, request.height);
  window.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
  window.webContents.session.webRequest.onBeforeRequest((details, callback) => {
    callback({ cancel: !details.url.startsWith('file:') && !details.url.startsWith('data:') });
  });
  try {
    trace('load HTML');
    await window.loadFile(request.html_path);
    // BrowserWindow runs a DOM, not a drawing surface. Wait for the embedded
    // fonts and images before Chromium captures the composited page.
    trace('wait for fonts and images');
    await window.webContents.executeJavaScript(`(async () => {
      await document.fonts.ready;
      await Promise.all(Array.from(document.images, image => image.decode()));
      await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      if (document.querySelector('canvas')) throw new Error('Overlay must use HTML/CSS and SVG');
      if (window.innerWidth !== 1920 || window.innerHeight !== 1080) throw new Error('Incorrect overlay viewport: ' + window.innerWidth + 'x' + window.innerHeight);
    })()`);
    // Fonts/layout readiness can precede an already queued empty paint. A tiny
    // temporary DOM marker proves the compositor has presented this document;
    // capture only the following frame where the marker has been removed.
    // No marker or drawing surface is present in the exported PNG.
    const paintAfter = (markerPresent: boolean, mutation: string): Promise<Buffer> => new Promise((resolve, reject) => {
      const painted = (_event: Electron.Event, _dirty: Electron.Rectangle, image: Electron.NativeImage): void => {
        const size = image.getSize();
        if (size.width !== request.width || size.height !== request.height) {
          window.webContents.invalidate();
          return;
        }
        const pixels = image.toBitmap(); // NativeImage supplies premultiplied BGRA.
        const hasMarker = pixels[0] === 55 && pixels[1] === 228 && pixels[2] === 18 && pixels[3] === 255;
        if (hasMarker !== markerPresent) return;
        window.webContents.removeListener('paint', painted);
        resolve(image.toPNG());
      };
      window.webContents.on('paint', painted);
      void window.webContents.executeJavaScript(mutation).then(() => window.webContents.invalidate()).catch((error: unknown) => {
        window.webContents.removeListener('paint', painted);
        reject(error);
      });
    });
    trace('wait for composited page');
    await paintAfter(true, `(() => {
      const marker = document.createElement('div'); marker.id = 'cliphub-paint-marker';
      marker.style.cssText = 'position:fixed;left:0;top:0;width:2px;height:2px;background:rgb(18,228,55);z-index:2147483647;pointer-events:none';
      document.documentElement.append(marker);
    })()`);
    trace('capture PNG');
    const png = await paintAfter(false, `document.getElementById('cliphub-paint-marker').remove()`);
    fs.mkdirSync(path.dirname(request.output_path), { recursive: true });
    fs.writeFileSync(request.output_path, png);
    trace('PNG saved');
  } finally {
    clearTimeout(timeout);
    window.destroy();
  }
  app.exit(0);
}
