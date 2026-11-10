'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayClock } from '@/lib/replay/clock';
import { demoMsOfTick } from '@/lib/replay/voice-sync';
import { useMatchVoice } from './use-match-voice';
import type { MatchVoiceControl } from './use-match-voice';
import { useVoicePlayback } from './use-voice-playback';

const VOLUME_KEY = 'cliphub.replay.voice-volume';

export type ReplayVoice = MatchVoiceControl & {
  matchId: string | null;
  clock: ReplayClock;
  /** The focused round's demo time at a clock position, in ms; null while no round is drawn. */
  demoMsAt: (seconds: number) => number | null;
  /** The offset playing now: the adjust bar's draft while it is open, else the saved one. */
  offsetMs: number;
  draft: number | null;
  setDraft: (ms: number | null) => void;
  volume: number;
  setVolume: (volume: number) => void;
  muted: boolean;
  setMuted: (muted: boolean) => void;
  mutedTracks: ReadonlySet<number>;
  toggleTrack: (track: number) => void;
};

function storedVolume(): number {
  try {
    const raw = Number(window.localStorage.getItem(VOLUME_KEY));
    return Number.isFinite(raw) && raw > 0 && raw <= 1 ? raw : 0.8;
  } catch {
    return 0.8;
  }
}

/** The Replay's team voice: the match's recording, how loud it plays and the sync being adjusted. */
export function useReplayVoice(options: {
  matchId: string | null;
  clock: ReplayClock;
  focused: CentreRound | null;
  tickOf: (round: CentreRound, seconds: number) => number;
  playing: boolean;
  speed: number;
}): ReplayVoice {
  const { matchId, clock, focused, tickOf, playing, speed } = options;
  const control = useMatchVoice(matchId);
  const [draft, setDraft] = useState<number | null>(null);
  const [volume, setVolumeState] = useState(0.8);
  const [muted, setMuted] = useState(false);
  const [mutedTracks, setMutedTracks] = useState<ReadonlySet<number>>(new Set());
  useEffect(() => setVolumeState(storedVolume()), []);
  useEffect(() => {
    setDraft(null);
    setMutedTracks(new Set());
  }, [matchId]);

  const setVolume = useCallback((next: number) => {
    setVolumeState(next);
    try {
      window.localStorage.setItem(VOLUME_KEY, String(next));
    } catch {
      // A blocked storage only forgets the level.
    }
  }, []);
  const toggleTrack = useCallback((track: number) => {
    setMutedTracks((current) => {
      const next = new Set(current);
      if (!next.delete(track)) next.add(track);
      return next;
    });
  }, []);

  const tickAt = useCallback((seconds: number) => (focused === null ? null : tickOf(focused, seconds)), [focused, tickOf]);
  const tickrate = focused?.timeline.tickrate ?? 64;
  const demoMsAt = useCallback(
    (seconds: number) => {
      const tick = tickAt(seconds);
      return tick === null ? null : demoMsOfTick(tick, tickrate);
    },
    [tickAt, tickrate],
  );
  const voice = control.state.kind === 'ready' && control.state.voice.present ? control.state.voice : null;
  const offsetMs = draft ?? voice?.sync.offset_ms ?? 0;
  const tracks = useMemo(() => voice?.tracks ?? [], [voice]);
  useVoicePlayback({
    matchId: voice === null ? null : matchId,
    tracks,
    clock,
    tickAt,
    tickrate,
    offsetMs,
    playing,
    speed,
    volume,
    muted,
    mutedTracks,
  });

  return { ...control, matchId, clock, demoMsAt, offsetMs, draft, setDraft, volume, setVolume, muted, setMuted, mutedTracks, toggleTrack };
}
