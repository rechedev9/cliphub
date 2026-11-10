'use client';

import { useState, type ReactNode } from 'react';
import { Headphones, Trash2, Volume2, VolumeX } from 'lucide-react';
import type { TacticalDocument } from '@/lib/api/tactical';
import type { VoicePhase, VoicePhaseTalk, VoiceSpeakerTalk, VoiceTrack } from '@/lib/api/voice';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';
import type { ReplayVoice } from './use-replay-voice';

export const PHASES: readonly { key: VoicePhase; field: keyof VoicePhaseTalk; label: string; tone: string }[] = [
  { key: 'freezetime', field: 'freezetime_ms', label: 'Tiempo de compra', tone: 'bg-fg-3' },
  { key: 'midround', field: 'midround_ms', label: 'Ronda', tone: 'bg-primary' },
  { key: 'post_plant', field: 'post_plant_ms', label: 'Tras plantar', tone: 'bg-warning' },
  { key: 'dead', field: 'dead_ms', label: 'Muerto', tone: 'bg-destructive' },
  { key: 'after_round', field: 'after_round_ms', label: 'Tras la ronda', tone: 'bg-surface-5' },
];

const NOBODY = 'nadie';

/** Who a track or a phrase belongs to: its player when tied to one, else its label. */
export type Speaker = { steamid64?: string; label: string };

function totalMs(talk: VoicePhaseTalk): number {
  return PHASES.reduce((sum, phase) => sum + talk[phase.field], 0);
}

function seconds(ms: number): string {
  return `${Math.round(ms / 1000)} s`;
}

/** Who spoke and when: one bar per speaker, split by round phase, on a scale shared by every bar. */
export function TalkBars({ speakers, names }: { speakers: readonly VoiceSpeakerTalk[]; names: (speaker: Speaker) => string }): ReactNode {
  const scale = Math.max(1, ...speakers.map(totalMs));
  const talking = speakers.filter((speaker) => totalMs(speaker) > 0);
  if (talking.length === 0) return <p className="text-meta text-fg-3">Nadie habló en esta ronda.</p>;
  return (
    <div className="flex flex-col gap-1.5">
      <ul className="flex flex-col gap-1">
        {talking.map((speaker) => (
          <li key={speaker.track} className="grid grid-cols-[minmax(0,5.5rem)_minmax(0,1fr)_2.5rem] items-center gap-2 text-meta">
            <span className="truncate text-fg-2">{names(speaker)}</span>
            <span className="flex h-2 overflow-hidden rounded-full bg-surface-0" aria-hidden>
              {PHASES.map((phase) => (
                <span key={phase.key} className={phase.tone} style={{ width: `${(speaker[phase.field] / scale) * 100}%` }} />
              ))}
            </span>
            <span className="text-right tabular-nums text-fg-1">{seconds(totalMs(speaker))}</span>
          </li>
        ))}
      </ul>
      <p className="flex flex-wrap gap-x-2 gap-y-0.5 text-[11px] text-fg-3">
        {PHASES.map((phase) => (
          <span key={phase.key} className="inline-flex items-center gap-1">
            <span className={cn('size-2 rounded-full', phase.tone)} aria-hidden />
            {phase.label}
          </span>
        ))}
      </p>
    </div>
  );
}

/** The match in one table: each speaker's talk time, and how much of it fell in the buy time. */
export function MatchTalkSummary({ totals, names }: { totals: readonly VoiceSpeakerTalk[]; names: (speaker: Speaker) => string }): ReactNode {
  const rows = totals.filter((speaker) => totalMs(speaker) > 0);
  if (rows.length === 0) return <p className="text-meta text-fg-3">La grabación no tiene voz dentro de las rondas.</p>;
  return (
    <table className="w-full text-meta">
      <thead className="text-fg-3">
        <tr>
          <th className="text-left font-normal">Quién</th>
          <th className="text-right font-normal">Total</th>
          <th className="text-right font-normal">Compra</th>
          <th className="text-right font-normal">Muerto</th>
        </tr>
      </thead>
      <tbody className="tabular-nums text-fg-1">
        {rows.map((speaker) => (
          <tr key={speaker.track}>
            <td className="max-w-24 truncate text-fg-2">{names(speaker)}</td>
            <td className="text-right">{seconds(totalMs(speaker))}</td>
            <td className="text-right">{seconds(speaker.freezetime_ms)}</td>
            <td className="text-right">{speaker.steamid64 ? seconds(speaker.dead_ms) : '·'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

/** Each track: mute it, and say which player speaks on it, which splits their talk into alive and dead. */
export function VoiceTracks({ voice, tracks, players }: { voice: ReplayVoice; tracks: readonly VoiceTrack[]; players: TacticalDocument['players'] }): ReactNode {
  const [confirm, setConfirm] = useState(false);
  const untied = tracks.some((track) => !track.steamid64);
  return (
    <div className="flex flex-col gap-1.5">
      {untied ? <p className="text-meta text-fg-2">Elige quién habla en cada pista: Studio lo recordará en las próximas grabaciones del equipo.</p> : null}
      <ul className="flex flex-col gap-1">
        {tracks.map((track) => {
          const off = voice.mutedTracks.has(track.track);
          const solo = !off && tracks.length > 1 && tracks.every((other) => other.track === track.track || voice.mutedTracks.has(other.track));
          return (
            <li key={track.track} className="flex items-center gap-1.5">
              <Button type="button" size="icon-xs" variant="ghost" aria-pressed={off} aria-label={off ? `Activar ${track.label}` : `Silenciar ${track.label}`} onClick={() => voice.toggleTrack(track.track)}>
                {off ? <VolumeX aria-hidden /> : <Volume2 aria-hidden />}
              </Button>
              {tracks.length > 1 ? (
                <Button type="button" size="icon-xs" variant="ghost" aria-pressed={solo} aria-label={solo ? 'Oír todas las pistas' : `Oír solo ${track.label}`} onClick={() => voice.soloTrack(track.track)}>
                  <Headphones aria-hidden className={solo ? 'text-primary' : undefined} />
                </Button>
              ) : null}
              <Select value={track.steamid64 || NOBODY} onValueChange={(value) => voice.nameTrack(track.track, { steamid64: value === NOBODY ? '' : value })}>
                <SelectTrigger aria-label={`Quién habla en ${track.label}`} className="h-8 min-w-0 flex-1 text-meta">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NOBODY}>{track.label}</SelectItem>
                  {players.map((player) => (
                    <SelectItem key={player.steamid64} value={player.steamid64}>
                      {player.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </li>
          );
        })}
      </ul>
      <div className="flex justify-end">
        {confirm ? (
          <span className="flex items-center gap-1">
            <span className="text-meta text-fg-2">¿Quitar la voz y su transcripción?</span>
            <Button type="button" size="xs" variant="destructive" loading={voice.busy} onClick={() => voice.remove()}>
              Quitar
            </Button>
            <Button type="button" size="xs" variant="ghost" onClick={() => setConfirm(false)}>
              No
            </Button>
          </span>
        ) : (
          <Button type="button" size="xs" variant="ghost" onClick={() => setConfirm(true)}>
            <Trash2 aria-hidden />
            Quitar voz
          </Button>
        )}
      </div>
    </div>
  );
}
