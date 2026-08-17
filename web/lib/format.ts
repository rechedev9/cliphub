import type { Match, Play, VideoStatus } from './api/types';

export function formatKd(n: number): string {
  return n.toFixed(2);
}

/** "de_dust2" -> "Dust2", "cs_office" -> "Office"; passes through anything unprefixed. */
export function prettyMapName(map: string): string {
  const stripped = map.replace(/^(de|cs)_/, '');
  return stripped.charAt(0).toUpperCase() + stripped.slice(1);
}

/** Tailwind text-colour class for an HLTV-1.0 rating, by performance band. */
export function ratingClass(rating: number): string {
  if (rating >= 1.15) return 'text-emerald-400';
  if (rating >= 0.95) return 'text-foreground';
  if (rating >= 0.8) return 'text-amber-400';
  return 'text-rose-400';
}

/** Tailwind background-colour class for a rating bar fill, matching ratingClass's bands. */
export function ratingBarClass(rating: number): string {
  if (rating >= 1.15) return 'bg-emerald-400';
  if (rating >= 0.95) return 'bg-foreground';
  if (rating >= 0.8) return 'bg-amber-400';
  return 'bg-rose-400';
}

/** 0-100 fill for a rating bar, scaled so a 2.0 rating (an elite pace) fills it. */
export function ratingBarPct(rating: number): number {
  return Math.min(100, Math.max(0, (rating / 2) * 100));
}

/** Relative time like "hace 2 h" / "hace 3 d" / "ahora mismo" from an ISO string or epoch ms. */
export function timeAgo(value: string | number): string {
  const then = typeof value === 'number' ? value : Date.parse(value);
  const diffSec = Math.max(0, (Date.now() - then) / 1000);

  if (diffSec < 60) return 'ahora mismo';
  const minutes = Math.floor(diffSec / 60);
  if (minutes < 60) return `hace ${minutes} min`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `hace ${hours} h`;
  const days = Math.floor(hours / 24);
  return `hace ${days} d`;
}

export function formatShortDate(value: string | number): string {
  const date = typeof value === 'number' ? new Date(value) : new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  const now = new Date();
  const madridYear = new Intl.DateTimeFormat('en', { year: 'numeric', timeZone: 'Europe/Madrid' });
  const sameYear = madridYear.format(date) === madridYear.format(now);
  return new Intl.DateTimeFormat('es-ES', {
    day: 'numeric',
    month: 'short',
    ...(sameYear ? {} : { year: 'numeric' }),
    timeZone: 'Europe/Madrid',
  }).format(date);
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

/** Remaining-availability countdown: "14h" or "13h 59m" or "12m". */
export function formatCountdown(sec: number): string {
  const total = Math.max(0, Math.floor(sec));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);

  if (hours <= 0) return `${minutes}m`;
  if (minutes === 0) return `${hours}h`;
  return `${hours}h ${minutes}m`;
}

/** Plan-order selection summary; one pick keeps its label, 2+ list distinct rounds. */
export function playsSelectionLabel(plays: Play[]): string | null {
  if (plays.length === 0) return null;
  if (plays.length === 1) return plays[0].label;
  const rounds = Array.from(new Set(plays.map((p) => p.round))).sort((a, b) => a - b);
  return `${plays.length} jugadas · Rondas ${rounds.join(', ')}`;
}

/** Spanish render status; queued/composing both read Editando. */
export function productStatusLabel(status: VideoStatus): string {
  switch (status) {
    case 'recording':
      return 'Capturando';
    case 'queued':
    case 'composing':
      return 'Editando';
    case 'ready':
      return 'Listo';
    case 'review_required':
      return 'Revisión necesaria';
    case 'failed':
      return 'Fallido';
  }
}
