import { customHudTheme } from './custom-hud.ts';

/** Mirrors hudthemes.Style: a nil radius keeps the base design's shape. */
export type HudStyle = { name: string; ct: string; t: string; background: string; radius?: number };
/** Mirrors hudthemes.Theme: a catalog design (base) copied with the user's style. */
export type UserHudTheme = HudStyle & { id: string; base: string };
/** Mirrors customhud.Options, the composition switches of overlays.hud_options. */
export type HudOptions = { compact_header?: boolean; hide_loadout?: boolean; round_win_banner?: boolean; win_probability?: boolean };
/** Mirrors customhud.Teams: names by the side each team started on. */
export type HudTeamNames = { ct_start?: string; t_start?: string };

export const HUD_THEMES_API = '/api/full-demo/hud-themes';
export const MAX_HUD_THEME_NAME = 40;
export const MAX_HUD_RADIUS = 15;
export const MAX_HUD_TEAM_NAME = 32;
const COLOR = /^[0-9A-F]{6}$/;
const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

export function isHudStyle(value: unknown): value is HudStyle {
  if (!record(value)) return false;
  const { name, ct, t, background, radius } = value;
  return typeof name === 'string' && name.trim() !== '' && [...name].length <= MAX_HUD_THEME_NAME
    && [ct, t, background].every((color) => typeof color === 'string' && COLOR.test(color))
    && (radius === undefined || (typeof radius === 'number' && Number.isInteger(radius) && radius >= 0 && radius <= MAX_HUD_RADIUS));
}

export function isHudOptions(value: unknown): value is HudOptions {
  return record(value) && Object.entries(value).every(([key, flag]) => ['compact_header', 'hide_loadout', 'round_win_banner', 'win_probability'].includes(key) && typeof flag === 'boolean');
}

export function isHudTeamNames(value: unknown): value is HudTeamNames {
  return record(value) && Object.entries(value).every(([key, name]) => ['ct_start', 't_start'].includes(key)
    && typeof name === 'string' && name === name.trim() && [...name].length <= MAX_HUD_TEAM_NAME);
}

function isUserHudTheme(value: unknown): value is UserHudTheme {
  return record(value) && typeof value.id === 'string' && UUID.test(value.id) && typeof value.base === 'string' && customHudTheme(value.base) !== undefined && isHudStyle(value);
}

/** Only the style fields, so a theme's id and dates never reach a plan. */
export function hudStyleOf(theme: HudStyle): HudStyle {
  return { name: theme.name, ct: theme.ct, t: theme.t, background: theme.background, ...(theme.radius === undefined ? {} : { radius: theme.radius }) };
}

export function sameHudStyle(a: HudStyle | undefined, b: HudStyle | undefined): boolean {
  return a !== undefined && b !== undefined && a.name === b.name && a.ct === b.ct && a.t === b.t && a.background === b.background && a.radius === b.radius;
}

/** A first copy of a catalog design: its colours and shape, a new name. */
export function duplicateStyle(base: string, current?: HudStyle): HudStyle {
  if (current) return { ...hudStyleOf(current), name: copyName(current.name) };
  const theme = customHudTheme(base);
  return { name: copyName(theme?.name ?? 'Tema'), ct: theme?.ct ?? '42C9E8', t: theme?.t ?? 'FFB32C', background: theme?.background ?? '101927' };
}

function copyName(name: string): string {
  return [...`${name} (copia)`].slice(0, MAX_HUD_THEME_NAME).join('');
}

/** `#aabbcc` from a colour input becomes the catalog's `AABBCC`. */
export function hudColor(input: string): string {
  return input.replace(/^#/, '').toUpperCase();
}

/** Without the switches, names or a style the static catalog preview is the same picture. */
export function needsDrawnPreview(style: HudStyle | undefined, options: HudOptions | undefined, names: HudTeamNames | undefined): boolean {
  return style !== undefined || Object.values(options ?? {}).some(Boolean) || Object.values(names ?? {}).some((name) => name !== '');
}

/** The orchestrator draws the example scene with the same renderer as the video. */
export function hudPreviewUrl({ base, style, options, names }: { base: string; style?: HudStyle; options?: HudOptions; names?: HudTeamNames }): string {
  const query = new URLSearchParams({ base });
  if (style) {
    query.set('ct', style.ct); query.set('t', style.t); query.set('background', style.background);
    if (style.radius !== undefined) query.set('radius', String(style.radius));
  }
  if (options?.compact_header) query.set('compact_header', '1');
  if (options?.hide_loadout) query.set('hide_loadout', '1');
  if (options?.round_win_banner) query.set('round_win_banner', '1');
  if (options?.win_probability) query.set('win_probability', '1');
  if (names?.ct_start) query.set('ct_name', names.ct_start);
  if (names?.t_start) query.set('t_name', names.t_start);
  return `${HUD_THEMES_API}/preview?${query.toString()}`;
}

/** The 9:16 Short band (scoreboard and alive counts) drawn by the same renderer as the video. */
export function verticalHudPreviewUrl(base: string): string {
  return `${HUD_THEMES_API}/preview?${new URLSearchParams({ base, vertical: '1' }).toString()}`;
}

async function call(path: string, init?: RequestInit): Promise<unknown> {
  const response = await fetch(path, { cache: 'no-store', ...init });
  const body: unknown = response.status === 204 ? null : await response.json().catch(() => null);
  if (!response.ok) {
    if (record(body) && body.code === 'hud_theme_limit') throw new Error('Ya tienes 10 temas, el máximo. Borra uno para crear otro.');
    throw new Error(record(body) && typeof body.error === 'string' ? body.error : 'No se pudo guardar el tema.');
  }
  return body;
}

function json(method: string, body: unknown): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
}

function theme(body: unknown): UserHudTheme {
  if (!isUserHudTheme(body)) throw new Error('Studio devolvió un tema que no se puede leer.');
  return body;
}

export async function listHudThemes(signal?: AbortSignal): Promise<{ themes: UserHudTheme[]; max: number }> {
  const body = await call(HUD_THEMES_API, { signal });
  if (!record(body) || !Array.isArray(body.themes) || typeof body.max !== 'number') throw new Error('Studio devolvió una lista de temas que no se puede leer.');
  return { themes: body.themes.filter(isUserHudTheme), max: body.max };
}

export async function createHudTheme(base: string, style: HudStyle): Promise<UserHudTheme> {
  return theme(await call(HUD_THEMES_API, json('POST', { base, ...hudStyleOf(style) })));
}

export async function updateHudTheme(id: string, style: HudStyle): Promise<UserHudTheme> {
  return theme(await call(`${HUD_THEMES_API}/${encodeURIComponent(id)}`, json('PUT', hudStyleOf(style))));
}

export async function deleteHudTheme(id: string): Promise<void> {
  await call(`${HUD_THEMES_API}/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
