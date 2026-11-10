'use client';

import { useEffect, useRef } from 'react';
import type { ReplayClock } from '@/lib/replay/clock';
import { voiceTrackAudioUrl } from '@/lib/api/voice';
import type { VoiceTrack } from '@/lib/api/voice';
import { audioSecondsAt, needsSeek, playbackRate, trackVolume } from '@/lib/replay/voice-sync';

export type VoicePlaybackOptions = {
  matchId: string | null;
  tracks: readonly VoiceTrack[];
  clock: ReplayClock;
  /** The focused round's demo tick at a clock position; null while no round is drawn. */
  tickAt: (seconds: number) => number | null;
  tickrate: number;
  offsetMs: number;
  playing: boolean;
  speed: number;
  volume: number;
  muted: boolean;
  mutedTracks: ReadonlySet<number>;
};

/**
 * Plays each voice track in an audio element that follows the Replay's clock:
 * play, pause, seek, speed and round changes all move it; outside the recording it is silent.
 */
export function useVoicePlayback(options: VoicePlaybackOptions): void {
  const { matchId, tracks, clock, playing, speed, volume, muted, mutedTracks, offsetMs, tickAt } = options;
  const elements = useRef(new Map<number, HTMLAudioElement>());
  const live = useRef(options);
  live.current = options;

  const trackKey = tracks.map((track) => `${track.track}:${track.duration_ms}`).join(',');
  useEffect(() => {
    const made = elements.current;
    if (matchId === null) return;
    for (const track of live.current.tracks) {
      const audio = new Audio(voiceTrackAudioUrl(matchId, track.track));
      audio.preload = 'auto';
      made.set(track.track, audio);
    }
    return () => {
      for (const audio of made.values()) {
        audio.pause();
        audio.removeAttribute('src');
        audio.load();
      }
      made.clear();
    };
    // trackKey stands for the tracks, so a new status object with the same tracks keeps the elements.
  }, [matchId, trackKey]);

  useEffect(() => {
    const follow = (seconds: number): void => {
      const now = live.current;
      const tick = now.tickAt(seconds);
      for (const track of now.tracks) {
        const audio = elements.current.get(track.track);
        if (audio === undefined) continue;
        audio.volume = trackVolume({ master: now.volume, muted: now.muted, trackMuted: now.mutedTracks.has(track.track) });
        audio.playbackRate = playbackRate(now.speed);
        const target = tick === null ? null : audioSecondsAt({ tick, tickrate: now.tickrate, offsetMs: now.offsetMs, durationMs: track.duration_ms });
        if (target === null || !now.playing) {
          if (!audio.paused) audio.pause();
          if (target !== null && needsSeek(audio.currentTime, target, false)) audio.currentTime = target;
          continue;
        }
        if (needsSeek(audio.currentTime, target, true)) audio.currentTime = target;
        // A refused play (no user gesture yet) leaves the track silent until the next one.
        if (audio.paused) audio.play().catch(() => undefined);
      }
    };
    follow(clock.seconds);
    return clock.subscribe(follow);
  }, [clock, trackKey]);

  // Pause, speed, volume and offset changes do not move the clock, so apply them now.
  useEffect(() => {
    clock.refresh();
  }, [clock, playing, speed, volume, muted, mutedTracks, offsetMs, tickAt]);
}
