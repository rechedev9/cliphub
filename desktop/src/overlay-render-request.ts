import * as path from 'node:path';

/** The landscape Full Demo still and the 9:16 Short still: the only frames the renderer draws. */
const OVERLAY_SIZES = [{ width: 1920, height: 1080 }, { width: 1080, height: 1920 }] as const;

export interface OverlayRenderRequest {
  html_path: string;
  output_path: string;
  width: (typeof OVERLAY_SIZES)[number]['width'];
  height: (typeof OVERLAY_SIZES)[number]['height'];
}

function absoluteFile(value: unknown, extension: string): value is string {
  return typeof value === 'string' && path.isAbsolute(value) && path.extname(value).toLowerCase() === extension;
}

export function parseOverlayRenderRequest(value: unknown): OverlayRenderRequest {
  if (typeof value !== 'object' || value === null) throw new Error('Invalid overlay render request');
  const htmlPath: unknown = Reflect.get(value, 'html_path');
  const outputPath: unknown = Reflect.get(value, 'output_path');
  const width: unknown = Reflect.get(value, 'width');
  const height: unknown = Reflect.get(value, 'height');
  const size = OVERLAY_SIZES.find((candidate) => candidate.width === width && candidate.height === height);
  if (!size || !absoluteFile(htmlPath, '.html') || !absoluteFile(outputPath, '.png')) {
    throw new Error('Overlay requires absolute HTML/PNG paths and a 1920x1080 or 1080x1920 viewport');
  }
  return { html_path: htmlPath, output_path: outputPath, width: size.width, height: size.height };
}
