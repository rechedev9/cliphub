import type { LibraryMatch } from './api/library.ts';
import type { VoiceRecording } from './api/voice-plugin.ts';

/** Matches a recording can go to: its map first when CS2 reported one, then by closeness in time. */
export function recordingMatchChoices(recording: VoiceRecording, matches: readonly LibraryMatch[], limit = 20): LibraryMatch[] {
  const started = Date.parse(recording.started_at);
  const map = recording.map ?? '';
  const distance = (match: LibraryMatch): number => Math.abs(Date.parse(match.played_at) - started);
  return [...matches]
    .sort((a, b) => Number(b.map === map) - Number(a.map === map) || distance(a) - distance(b))
    .slice(0, limit);
}

/** "12/11 21:40 · de_mirage · 4 pistas · 38:12", the line that names a recording in Ajustes. */
export function recordingTitle(recording: VoiceRecording): string {
  const when = new Date(recording.started_at).toLocaleString('es-ES', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  const tracks = recording.tracks.length === 1 ? '1 pista' : `${recording.tracks.length} pistas`;
  return [when, recording.map, tracks, clock(recording.duration_ms)].filter(Boolean).join(' · ');
}

export function matchLabel(match: LibraryMatch): string {
  const when = new Date(match.played_at).toLocaleString('es-ES', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  const teams = match.teams.map((team) => `${team.team_name || team.clan_name || team.key} ${team.score}`).join(' - ');
  return `${when} · ${match.map} · ${teams}`;
}

function clock(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = String(total % 60).padStart(2, '0');
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, '0')}:${seconds}` : `${minutes}:${seconds}`;
}
