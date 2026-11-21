import type { EditConfig } from './api/types.ts';
import { apiFetch } from './api/edition-fetch.ts';

/** Mirrors brandkit.ImageRef: an uploaded overlay image by id and content hash. */
export type BrandImage = { id: string; sha256: string };
export const BRAND_CORNERS = ['top-left', 'top-right', 'bottom-left', 'bottom-right'] as const;
export type BrandCorner = (typeof BRAND_CORNERS)[number];
/** Mirrors brandkit.Kit; a missing logo or banner leaves its key out, as Go's omitempty does. */
export type BrandKit = {
  logo?: BrandImage;
  watermark_corner: BrandCorner;
  watermark_opacity: number;
  banner?: BrandImage;
  banner_start_seconds: number;
  banner_end_seconds?: number;
};
/** Mirrors brandkit.Watermark and brandkit.Banner, the copies a render or a plan keeps. */
export type BrandWatermark = { image: BrandImage; corner: BrandCorner; opacity: number };
export type BrandBanner = { image: BrandImage; start_seconds: number; end_seconds?: number };
/** Mirrors recapplan.BrandOverlay; Go omits it when both are empty. */
export type BrandOverlay = { watermark?: BrandWatermark; banner?: BrandBanner };

export const BRAND_KIT_API = '/api/brand-kit';
const OVERLAY_IMAGES_API = '/api/full-demo/overlay-images';
export const MIN_BRAND_OPACITY = 0.1;
export const MAX_BRAND_SECONDS = 43200;
export const MAX_BRAND_IMAGE_BYTES = 10 * 1024 * 1024;
export const DEFAULT_BRAND_KIT: BrandKit = { watermark_corner: 'top-right', watermark_opacity: 0.8, banner_start_seconds: 0 };
export const BRAND_CORNER_LABELS: Record<BrandCorner, string> = {
  'top-left': 'Arriba a la izquierda',
  'top-right': 'Arriba a la derecha',
  'bottom-left': 'Abajo a la izquierda',
  'bottom-right': 'Abajo a la derecha',
};
const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const SHA256 = /^[0-9a-f]{64}$/;

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function onlyKeys(value: Record<string, unknown>, keys: readonly string[]): boolean {
  return Object.keys(value).every((key) => keys.includes(key));
}

function seconds(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= MAX_BRAND_SECONDS;
}

function opacity(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= MIN_BRAND_OPACITY && value <= 1;
}

export function isBrandCorner(value: unknown): value is BrandCorner {
  return typeof value === 'string' && BRAND_CORNERS.some((corner) => corner === value);
}

export function isBrandImage(value: unknown): value is BrandImage {
  return record(value) && onlyKeys(value, ['id', 'sha256'])
    && typeof value.id === 'string' && UUID.test(value.id) && typeof value.sha256 === 'string' && SHA256.test(value.sha256);
}

function validWindow(start: unknown, end: unknown): boolean {
  return seconds(start) && (end === undefined || (seconds(end) && (end === 0 || end > start)));
}

export function isBrandWatermark(value: unknown): value is BrandWatermark {
  return record(value) && onlyKeys(value, ['image', 'corner', 'opacity']) && isBrandImage(value.image) && isBrandCorner(value.corner) && opacity(value.opacity);
}

export function isBrandBanner(value: unknown): value is BrandBanner {
  return record(value) && onlyKeys(value, ['image', 'start_seconds', 'end_seconds']) && isBrandImage(value.image) && validWindow(value.start_seconds, value.end_seconds);
}

export function isBrandOverlay(value: unknown): value is BrandOverlay {
  return record(value) && onlyKeys(value, ['watermark', 'banner'])
    && (value.watermark === undefined || isBrandWatermark(value.watermark))
    && (value.banner === undefined || isBrandBanner(value.banner));
}

/** Reads the server's kit; the response also carries `updated_at` and `corners`, which the form does not keep. */
export function parseBrandKit(value: unknown): BrandKit | undefined {
  if (!record(value)) return undefined;
  const { logo, watermark_corner: corner, watermark_opacity: alpha, banner, banner_start_seconds: start, banner_end_seconds: end } = value;
  if ((logo !== undefined && !isBrandImage(logo)) || (banner !== undefined && !isBrandImage(banner))) return undefined;
  if (!isBrandCorner(corner) || !opacity(alpha) || !validWindow(start, end) || !seconds(start)) return undefined;
  return {
    ...(logo ? { logo } : {}),
    watermark_corner: corner,
    watermark_opacity: alpha,
    ...(banner ? { banner } : {}),
    banner_start_seconds: start,
    ...(seconds(end) && end > 0 ? { banner_end_seconds: end } : {}),
  };
}

/** The Spanish reason a kit cannot be saved, or null when the server will accept it. */
export function brandKitProblem(kit: BrandKit): string | null {
  if (!opacity(kit.watermark_opacity)) return 'La opacidad debe estar entre el 10 % y el 100 %.';
  if (!seconds(kit.banner_start_seconds)) return 'El segundo de entrada del banner no es válido.';
  const end = kit.banner_end_seconds;
  if (end !== undefined && end !== 0 && (!seconds(end) || end <= kit.banner_start_seconds)) return 'El banner debe salir después de entrar.';
  return null;
}

export function brandWatermarkOf(kit: BrandKit): BrandWatermark | undefined {
  return kit.logo ? { image: kit.logo, corner: kit.watermark_corner, opacity: kit.watermark_opacity } : undefined;
}

export function brandBannerOf(kit: BrandKit): BrandBanner | undefined {
  if (!kit.banner) return undefined;
  const end = kit.banner_end_seconds;
  return { image: kit.banner, start_seconds: kit.banner_start_seconds, ...(end ? { end_seconds: end } : {}) };
}

/** "del segundo 2 al 6" or "desde el segundo 2 hasta el final", as the MCP summary says it. */
export function bannerWindowLabel(start: number, end?: number): string {
  const format = (value: number): string => String(value).replace('.', ',');
  return end ? `Del segundo ${format(start)} al ${format(end)}` : `Desde el segundo ${format(start)} hasta el final`;
}

/** The render rejects both banners together, so turning one on turns the other off. */
export function exclusiveBanner(previous: EditConfig, next: EditConfig): EditConfig {
  if (next.brandBanner && !previous.brandBanner && next.keyDropStyle) {
    return { ...next, keyDropFamily: '', keyDropStyle: '' };
  }
  if (next.keyDropStyle && next.keyDropStyle !== previous.keyDropStyle && next.brandBanner) {
    return { ...next, brandBanner: false };
  }
  return next;
}

export function brandImageUrl(image: BrandImage): string {
  return `${OVERLAY_IMAGES_API}/${image.id}`;
}

async function call(init?: RequestInit): Promise<BrandKit> {
  const response = await apiFetch(BRAND_KIT_API, { cache: 'no-store', ...init });
  const body: unknown = await response.json().catch(() => null);
  if (!response.ok) throw new Error(record(body) && typeof body.error === 'string' ? body.error : 'No se pudo leer el kit de marca.');
  const kit = parseBrandKit(body);
  if (!kit) throw new Error('Studio devolvió un kit de marca que no se puede leer.');
  return kit;
}

export function getBrandKit(signal?: AbortSignal): Promise<BrandKit> {
  return call({ signal });
}

export function saveBrandKit(kit: BrandKit): Promise<BrandKit> {
  const problem = brandKitProblem(kit);
  if (problem) return Promise.reject(new Error(problem));
  return call({ method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(kit) });
}

/** Uploads a PNG or JPG through the overlay image store the plan and the render already verify. */
export async function uploadBrandImage(file: File, signal?: AbortSignal): Promise<BrandImage> {
  if (!/\.(png|jpe?g)$/i.test(file.name)) throw new Error('Elige una imagen PNG o JPG.');
  if (file.size === 0 || file.size > MAX_BRAND_IMAGE_BYTES) throw new Error('La imagen debe ocupar entre 1 byte y 10 MB.');
  const form = new FormData();
  form.set('image', file);
  const response = await apiFetch(OVERLAY_IMAGES_API, { method: 'POST', body: form, signal });
  const body: unknown = await response.json().catch(() => null);
  if (!response.ok) throw new Error(record(body) && typeof body.error === 'string' ? body.error : 'No se pudo subir la imagen.');
  const ref = record(body) ? { id: body.id, sha256: body.sha256 } : null;
  if (!isBrandImage(ref)) throw new Error('Studio no pudo verificar la imagen subida.');
  return ref;
}
