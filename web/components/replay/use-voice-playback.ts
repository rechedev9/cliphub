'use client';

import { useEffect, useRef } from 'react';
import type { ReplayClock } from '@/lib/replay/clock';
import { audioSecondsAt, needsSeek, playbackRate, trackVolume } from '@/lib/replay/voice-sync';

/** One audio file on the Replay's clock; `durationMs` is Infinity when the file's own end is the limit. */
export type PlaybackTrack = { id: string; url: string; durationMs: number; muted: boolean };

export type VoicePlaybackOptions = {
  tracks: readonly PlaybackTrack[];
  clock: ReplayClock;
  /** The focused round's demo tick at a clock position; null while no round is drawn. */
  tickAt: (seconds: number) => number | null;
  tickrate: number;
  offsetMs: number;
  playing: boolean;
  speed: number;
  volume: number;
  muted: boolean;
  /** Ticks at which every track stays silent, such as outside freeze time. */
  silentAt: (tick: number) => boolean;
};

/**
 * Plays each track in an audio element that follows the Replay's clock:
 * play, pause, seek, speed and round changes all move it; outside the recording it is silent.
 */
export function useVoicePlayback(options: VoicePlaybackOptions): void {
  const { tracks, clock, playing, speed, volume, muted, offsetMs, tickAt, silentAt } = options;
  const elements = useRef(new Map<string, HTMLAudioElement>());
  const live = useRef(options);
  live.current = options;

  const trackKey = tracks.map((track) => `${track.id}:${track.url}:${track.durationMs}`).join(',');
  useEffect(() => {
    const made = elements.current;
    for (const track of live.current.tracks) {
      const audio = new Audio(track.url);
      audio.preload = 'auto';
      made.set(track.id, audio);
    }
    return () => {
      for (const audio of made.values()) {
        audio.pause();
        audio.removeAttribute('src');
        audio.load();
      }
      made.clear();
    };
    // trackKey stands for the tracks, so a new array with the same files keeps the elements.
  }, [trackKey]);

  useEffect(() => {
    const follow = (seconds: number): void => {
      const now = live.current;
      const tick = now.tickAt(seconds);
      const silent = tick !== null && now.silentAt(tick);
      for (const track of now.tracks) {
        const audio = elements.current.get(track.id);
        if (audio === undefined) continue;
        audio.volume = trackVolume({ master: now.volume, muted: now.muted || silent, trackMuted: track.muted });
        audio.playbackRate = playbackRate(now.speed);
        const target = tick === null ? null : audioSecondsAt({ tick, tickrate: now.tickrate, offsetMs: now.offsetMs, durationMs: track.durationMs });
        if (target === null || !now.playing) {
          if (!audio.paused) audio.pause();
          if (target !== null && needsSeek({ current: audio.currentTime, target, playing: false })) audio.currentTime = target;
          continue;
        }
        if (needsSeek({ current: audio.currentTime, target, playing: true })) audio.currentTime = target;
        // A refused play (no user gesture yet) leaves the track silent until the next one.
        if (audio.paused) audio.play().catch(() => undefined);
      }
    };
    follow(clock.seconds);
    return clock.subscribe(follow);
  }, [clock, trackKey]);

  // Pause, speed, volume, mute and offset changes do not move the clock, so apply them now.
  useEffect(() => {
    clock.refresh();
  }, [clock, playing, speed, volume, muted, tracks, offsetMs, tickAt, silentAt]);
}
