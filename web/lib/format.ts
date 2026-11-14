import type { Match, Play } from './api/types';

/** "de_dust2" -> "Dust2", "cs_office" -> "Office"; passes through anything unprefixed. */
export function prettyMapName(map: string): string {
  const stripped = map.replace(/^(de|cs)_/, '');
  return stripped.charAt(0).toUpperCase() + stripped.slice(1);
}

const DECIMAL_FORMATS = new Map<number, Intl.NumberFormat>();

/** A number with a fixed count of decimals and the Spanish decimal comma: `formatDecimal(1.5, 2)` is "1,50". */
export function formatDecimal(value: number, digits: number): string {
  let format = DECIMAL_FORMATS.get(digits);
  if (format === undefined) {
    format = new Intl.NumberFormat('es-ES', { minimumFractionDigits: digits, maximumFractionDigits: digits });
    DECIMAL_FORMATS.set(digits, format);
  }
  return format.format(value);
}

/** A playback rate or gain such as 0,25× with as many decimals as it has. */
export function formatRate(value: number): string {
  return `${String(value).replace('.', ',')}×`;
}

/** 0-100 fill for a rating bar, scaled so a 2.0 rating (an elite pace) fills it. */
export function ratingBarPct(rating: number): number {
  return Math.min(100, Math.max(0, (rating / 2) * 100));
}

/** Past this many days "hace N d" stops being readable ("hace 381 d") and the calendar date takes over. */
const RELATIVE_DAYS_LIMIT = 30;

/**
 * Relative time like "hace 2 h" / "hace 3 d" / "ahora mismo" from an ISO string
 * or epoch ms. Older than a month it falls back to the short calendar date.
 */
export function timeAgo(value: string | number): string {
  const then = typeof value === 'number' ? value : Date.parse(value);
  const diffSec = Math.max(0, (Date.now() - then) / 1000);

  if (diffSec < 60) return 'ahora mismo';
  const minutes = Math.floor(diffSec / 60);
  if (minutes < 60) return `hace ${minutes} min`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `hace ${hours} h`;
  const days = Math.floor(hours / 24);
  if (days < RELATIVE_DAYS_LIMIT) return `hace ${days} d`;
  return formatShortDate(then);
}

// Built once: a formatter costs far more than a format, and the replay dates every match on each render.
let shortDateFormats: { year: Intl.DateTimeFormat; day: Intl.DateTimeFormat; dayAndYear: Intl.DateTimeFormat } | null = null;

function shortDateFormatters(): NonNullable<typeof shortDateFormats> {
  shortDateFormats ??= {
    year: new Intl.DateTimeFormat('en', { year: 'numeric', timeZone: 'Europe/Madrid' }),
    day: new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', timeZone: 'Europe/Madrid' }),
    dayAndYear: new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Europe/Madrid' }),
  };
  return shortDateFormats;
}

export function formatShortDate(value: string | number): string {
  const date = typeof value === 'number' ? new Date(value) : new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  const formats = shortDateFormatters();
  const sameYear = formats.year.format(date) === formats.year.format(new Date());
  return (sameYear ? formats.day : formats.dayAndYear).format(date);
}

/** Imported demos expose their import timestamp, not a fabricated play time. */
export function matchDateLabel(match: Pick<Match, 'playedAt' | 'source'>): string {
  if (match.source !== 'upload') return timeAgo(match.playedAt);
  const date = new Date(match.playedAt);
  if (Number.isNaN(date.getTime())) return 'demo importada';
  return `importada el ${new Intl.DateTimeFormat('es-ES', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).format(date)}`;
}

/** Plan-order selection summary; one pick keeps its label, 2+ list distinct rounds. */
export function playsSelectionLabel(plays: Play[]): string | null {
  if (plays.length === 0) return null;
  if (plays.length === 1) return plays[0].label;
  const rounds = Array.from(new Set(plays.map((p) => p.round))).sort((a, b) => a - b);
  return `${plays.length} jugadas · Rondas ${rounds.join(', ')}`;
}
