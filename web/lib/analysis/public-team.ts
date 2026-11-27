import type { LibraryTeam } from '../api/library.ts';
import type { ProTeamCount, ProTeamImport } from '../api/pro-matches.ts';

export const PUBLIC_TEAM_DAYS = 120;

const MAX_PUBLIC_BATCHES = 60;

export type PublicSyncProgress = { name: string; held: number; total: number; imported: number; teamId: string | null };

export function proTeamKey(name: string): string {
  const key = name.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '');
  return key.startsWith('team') && key.length >= 6 ? key.slice(4) : key;
}

function teamKeys(team: Pick<LibraryTeam, 'name' | 'aliases'>): string[] {
  return [team.name, ...team.aliases].map(proTeamKey).filter((key) => key !== '');
}

export function publicTeamFor(team: Pick<LibraryTeam, 'name' | 'aliases'>, teams: readonly ProTeamCount[]): ProTeamCount | null {
  const keys = new Set(teamKeys(team));
  return teams.find((entry) => keys.has(proTeamKey(entry.name))) ?? null;
}

export function publicOnlyTeams(library: readonly Pick<LibraryTeam, 'name' | 'aliases' | 'match_count'>[], teams: readonly ProTeamCount[]): ProTeamCount[] {
  const held = new Set(library.filter((team) => team.match_count > 0).flatMap(teamKeys));
  return teams.filter((entry) => !held.has(proTeamKey(entry.name))).sort((a, b) => a.name.localeCompare(b.name, 'es'));
}

export type PublicSyncOptions = {
  hltvId: number;
  call: (hltvId: number) => Promise<ProTeamImport>;
  onBatch: (progress: PublicSyncProgress) => boolean;
};

export async function syncPublicTeam({ hltvId, call, onBatch }: PublicSyncOptions): Promise<PublicSyncProgress> {
  let progress: PublicSyncProgress = { name: '', held: 0, total: 0, imported: 0, teamId: null };
  let left = Number.POSITIVE_INFINITY;
  for (let batch = 0; batch < MAX_PUBLIC_BATCHES; batch += 1) {
    const res = await call(hltvId);
    progress = { name: res.name, held: res.held, total: res.total, imported: progress.imported + res.imported, teamId: res.team_id ?? progress.teamId };
    if (!onBatch(progress) || res.remaining === 0 || res.remaining >= left) return progress;
    left = res.remaining;
  }
  return progress;
}

export function publicSyncLine(progress: PublicSyncProgress, done: boolean): string {
  if (!done) return progress.total === 0 ? `Trayendo las partidas públicas de ${progress.name}…` : `Trayendo las partidas públicas de ${progress.name}: ${progress.held} de ${progress.total} mapas`;
  if (progress.total === 0) return `${progress.name} no tiene partidas públicas en los últimos ${PUBLIC_TEAM_DAYS} días.`;
  if (progress.held === progress.total) return `Incluye los ${progress.total} mapas públicos de ${progress.name} de los últimos ${PUBLIC_TEAM_DAYS} días.`;
  return `Incluye ${progress.held} de ${progress.total} mapas públicos de ${progress.name} de los últimos ${PUBLIC_TEAM_DAYS} días. Los que faltan los borraste o el servidor no los pudo enviar.`;
}
