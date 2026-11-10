'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { demoVoiceAudioUrl, voiceTrackAudioUrl } from '@/lib/api/voice';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayClock } from '@/lib/replay/clock';
import { demoMsOfTick, silentOutsideFreeze } from '@/lib/replay/voice-sync';
import { speakingSteamIds, type SpeakingTrack } from '@/lib/replay/voice-speaking';
import { useDemoVoice } from './use-demo-voice';
import type { DemoVoiceControl } from './use-demo-voice';
import { useMatchVoice } from './use-match-voice';
import type { MatchVoiceControl } from './use-match-voice';
import { useVoicePlayback } from './use-voice-playback';
import type { PlaybackTrack } from './use-voice-playback';

const VOLUME_KEY = 'cliphub.replay.voice-volume';
const NO_SLOTS: ReadonlySet<number> = new Set();

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
  /** Hears only this track; again on a soloed track brings every track back. */
  soloTrack: (track: number) => void;
  /** The focused round's players heard at a tick, by slot, for the mark on the map. */
  speakingAt: (round: CentreRound, tick: number) => ReadonlySet<number>;
  /** Only freeze time: the comms list keeps its phrases and playback goes quiet once the round is live. */
  freezeOnly: boolean;
  setFreezeOnly: (on: boolean) => void;
  /** The demo's own in-game voice, which plays at offset 0. */
  demoVoice: DemoVoiceControl;
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
  const demoVoice = useDemoVoice(matchId);
  const [draft, setDraft] = useState<number | null>(null);
  const [volume, setVolumeState] = useState(0.8);
  const [muted, setMuted] = useState(false);
  const [mutedTracks, setMutedTracks] = useState<ReadonlySet<number>>(new Set());
  const [freezeOnly, setFreezeOnly] = useState(false);
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
  const freezeEndTick = focused?.round.tick_freeze_end ?? null;
  const silentAt = useCallback((tick: number) => silentOutsideFreeze({ tick, freezeOnly, freezeEndTick }), [freezeEndTick, freezeOnly]);
  const voice = control.state.kind === 'ready' && control.state.voice.present ? control.state.voice : null;
  const offsetMs = draft ?? voice?.sync.offset_ms ?? 0;
  const trackNumbers = useMemo(() => voice?.tracks.map((track) => track.track) ?? [], [voice]);
  const soloTrack = useCallback(
    (track: number) => {
      setMutedTracks((current) => {
        const others = trackNumbers.filter((n) => n !== track);
        const soloed = !current.has(track) && others.every((n) => current.has(n));
        return soloed ? new Set() : new Set(others);
      });
    },
    [trackNumbers],
  );
  const comms = control.state.kind === 'ready' ? control.state.comms : null;
  const speakingTracks = useMemo<SpeakingTrack[]>(() => {
    if (voice === null || comms === null) return [];
    const speech = new Map(comms.tracks.map((track) => [track.track, track.speech]));
    return voice.tracks.map((track) => ({ track: track.track, steamid64: track.steamid64, speech: speech.get(track.track) ?? [] }));
  }, [comms, voice]);
  const speakingAt = useCallback(
    (round: CentreRound, tick: number): ReadonlySet<number> => {
      if (muted || speakingTracks.length === 0 || round.matchId !== matchId || silentAt(tick)) return NO_SLOTS;
      const audioMs = demoMsOfTick(tick, round.timeline.tickrate) - offsetMs;
      const ids = speakingSteamIds({ tracks: speakingTracks, muted: mutedTracks, audioMs });
      if (ids.size === 0) return NO_SLOTS;
      return new Set(round.doc.players.filter((player) => ids.has(player.steamid64)).map((player) => player.slot));
    },
    [matchId, muted, mutedTracks, offsetMs, silentAt, speakingTracks],
  );
  const teamTracks = useMemo<PlaybackTrack[]>(
    () =>
      voice === null || matchId === null
        ? []
        : voice.tracks.map((track) => ({
            id: `team-${track.track}`,
            url: voiceTrackAudioUrl(matchId, track.track),
            durationMs: track.duration_ms,
            muted: mutedTracks.has(track.track),
          })),
    [matchId, mutedTracks, voice],
  );
  const demoTracks = useMemo<PlaybackTrack[]>(
    () =>
      !demoVoice.on || demoVoice.demo === null || matchId === null
        ? []
        : demoVoice.demo.tracks.map((track) => ({
            id: `demo-${track.steamid64}`,
            url: demoVoiceAudioUrl(matchId, track.steamid64),
            durationMs: Number.POSITIVE_INFINITY,
            muted: false,
          })),
    [demoVoice.demo, demoVoice.on, matchId],
  );
  const shared = { clock, tickAt, tickrate, playing, speed, volume, muted, silentAt };
  useVoicePlayback({ ...shared, tracks: teamTracks, offsetMs });
  useVoicePlayback({ ...shared, tracks: demoTracks, offsetMs: 0 });

  return {
    ...control,
    matchId,
    clock,
    demoMsAt,
    offsetMs,
    draft,
    setDraft,
    volume,
    setVolume,
    muted,
    setMuted,
    mutedTracks,
    toggleTrack,
    soloTrack,
    speakingAt,
    freezeOnly,
    setFreezeOnly,
    demoVoice,
  };
}
