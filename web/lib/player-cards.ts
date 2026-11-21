import type { EditConfig } from './api/types.ts';
import { apiFetch } from './api/edition-fetch.ts';

/** The three independent card slots of a one-player 9:16 Short. */
export const PLAYER_CARD_SLOTS = [
  { key: 'openingCard', label: 'Apertura' },
  { key: 'endCard', label: 'Tarjeta final' },
  { key: 'coverCard', label: 'Portada' },
] as const;

export type PlayerCardSlot = (typeof PLAYER_CARD_SLOTS)[number]['key'];

/** Mirrors playercard.Unavailable for a Short that follows several players. */
export const MULTI_PLAYER_CARD_REASON = 'Las cartas de jugador son para un Short de un solo jugador, y este sigue a varios.';

/** A template from the catalog the orchestrator serves (internal/playercard). */
export type PlayerCardTemplate = { id: string; name: string; description: string };

export type PlayerCardImage = '' | 'upload' | 'faceit';

export type PlayerCardInfo = {
  templates: PlayerCardTemplate[];
  available: boolean;
  reason: string;
  player: { name: string; image: PlayerCardImage } | null;
};

const PLAYER_CARD_ID = /^[a-z][a-z0-9-]{0,31}$/;

/** A well-formed template id; the orchestrator checks it against its catalog. */
export function isPlayerCardId(value: unknown): value is string {
  return typeof value === 'string' && PLAYER_CARD_ID.test(value);
}

export function hasPlayerCards(edit: EditConfig): boolean {
  return PLAYER_CARD_SLOTS.some((slot) => edit[slot.key] !== undefined);
}

/** The edit with no card in any slot. */
export function withoutPlayerCards(edit: EditConfig): EditConfig {
  if (!hasPlayerCards(edit)) return edit;
  const next: EditConfig = { ...edit };
  for (const slot of PLAYER_CARD_SLOTS) delete next[slot.key];
  return next;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

/** Reads GET /api/demos/{jobId}/player-card; null for a body of another shape. */
export function parsePlayerCardInfo(value: unknown): PlayerCardInfo | null {
  if (typeof value !== 'object' || value === null) return null;
  const templates = Reflect.get(value, 'templates');
  if (!Array.isArray(templates)) return null;
  const catalog: PlayerCardTemplate[] = [];
  for (const entry of templates) {
    if (typeof entry !== 'object' || entry === null) continue;
    const id = Reflect.get(entry, 'id');
    if (isPlayerCardId(id)) catalog.push({ id, name: text(Reflect.get(entry, 'name')) || id, description: text(Reflect.get(entry, 'description')) });
  }
  const player = Reflect.get(value, 'player');
  let card: PlayerCardInfo['player'] = null;
  if (typeof player === 'object' && player !== null) {
    const image = Reflect.get(player, 'image');
    card = { name: text(Reflect.get(player, 'name')), image: image === 'upload' || image === 'faceit' ? image : '' };
  }
  return { templates: catalog, available: Reflect.get(value, 'available') === true, reason: text(Reflect.get(value, 'reason')), player: card };
}

/** The edit with this template in the slot; null empties it. */
export function withPlayerCard(edit: EditConfig, slot: PlayerCardSlot, id: string | null): EditConfig {
  const next: EditConfig = { ...edit };
  if (id === null) delete next[slot];
  else next[slot] = id;
  return next;
}

function cardApi(jobId: string, suffix = ''): string {
  return `/api/demos/${encodeURIComponent(jobId)}/player-card${suffix}`;
}

/** The SVG of the job's card in one template; version changes when the player's image does. */
export function playerCardPreviewUrl(jobId: string, templateId: string, version: number): string {
  return `${cardApi(jobId, '/preview')}?template=${encodeURIComponent(templateId)}&v=${version}`;
}

async function failure(response: Response, fallback: string): Promise<Error> {
  const body: unknown = await response.json().catch(() => null);
  const error = typeof body === 'object' && body !== null ? Reflect.get(body, 'error') : undefined;
  return new Error(typeof error === 'string' && error !== '' ? error : fallback);
}

export async function fetchPlayerCardInfo(jobId: string, signal?: AbortSignal): Promise<PlayerCardInfo> {
  const response = await apiFetch(cardApi(jobId), { cache: 'no-store', signal });
  if (!response.ok) throw await failure(response, 'No se pudieron leer las cartas de jugador.');
  const info = parsePlayerCardInfo(await response.json());
  if (info === null) throw new Error('Studio devolvió unas cartas de jugador ilegibles.');
  return info;
}

/** Keeps a PNG or JPG for the job's player: every later card of that player shows it. */
export async function uploadPlayerCardImage(jobId: string, file: File): Promise<void> {
  if (!/\.(png|jpe?g)$/i.test(file.name)) throw new Error('Elige una imagen PNG o JPG.');
  if (file.size === 0 || file.size > 10 * 1024 * 1024) throw new Error('La imagen debe ocupar entre 1 byte y 10 MB.');
  const form = new FormData();
  form.set('image', file);
  const response = await apiFetch(cardApi(jobId, '/image'), { method: 'POST', body: form });
  if (!response.ok) throw await failure(response, 'No se pudo subir la imagen.');
}

export async function removePlayerCardImage(jobId: string): Promise<void> {
  const response = await apiFetch(cardApi(jobId, '/image'), { method: 'DELETE' });
  if (!response.ok) throw await failure(response, 'No se pudo quitar la imagen.');
}
