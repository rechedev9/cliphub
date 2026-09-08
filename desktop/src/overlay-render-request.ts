import * as path from 'node:path';

export interface OverlayRenderRequest {
  html_path: string;
  output_path: string;
  width: 1920;
  height: 1080;
}

export function parseOverlayRenderRequest(value: unknown): OverlayRenderRequest {
  if (typeof value !== 'object' || value === null) throw new Error('Invalid overlay render request');
  const input = value as Record<string, unknown>;
  if (input.width !== 1920 || input.height !== 1080
    || typeof input.html_path !== 'string' || !path.isAbsolute(input.html_path)
    || typeof input.output_path !== 'string' || !path.isAbsolute(input.output_path)
    || path.extname(input.html_path).toLowerCase() !== '.html'
    || path.extname(input.output_path).toLowerCase() !== '.png') {
    throw new Error('Overlay requires absolute HTML/PNG paths and a 1920x1080 viewport');
  }
  return input as unknown as OverlayRenderRequest;
}
