import type { StreamClipRange } from './api/streams';
import { streamFadeEnvelope } from './stream-playback.ts';

const DEFAULT_MUSIC_VOLUME = 0.25;
const MUSIC_SEEK_DRIFT_SECONDS = 0.25;

type MusicSelection = { url: string; volume: number };

/**
 * Owns the Web Audio graph for one fresh stream playback video element.
 * Construction is side-effect free so the element keeps its direct audio path
 * until resume() runs from a user gesture.
 */
export class StreamAudioMixer {
  private readonly video: HTMLVideoElement;
  private readonly onError: () => void;
  private context: AudioContext | null = null;
  private sourceNode: MediaElementAudioSourceNode | null = null;
  private sourceGain: GainNode | null = null;
  private musicElement: HTMLAudioElement | null = null;
  private musicURL: string | null = null;
  private musicNode: MediaElementAudioSourceNode | null = null;
  private musicGain: GainNode | null = null;
  private clip: StreamClipRange | null = null;
  private music: MusicSelection | null = null;
  private sourceSeconds = 0;
  private playing = false;
  private musicPlayInFlight = false;
  private musicPlayToken = 0;
  private musicNeedsSeek = false;
  private disposed = false;
  private generation = 0;

  constructor(video: HTMLVideoElement, onError: () => void) {
    this.video = video;
    this.onError = onError;
  }

  configure(clip: StreamClipRange | null, music: MusicSelection | null): void {
    if (this.disposed) return;
    const previousClipKey = clipIdentity(this.clip);
    const previousMusicURL = this.music?.url ?? null;
    this.clip = clip;
    this.music = validMusic(music);
    if (clipIdentity(clip) !== previousClipKey) {
      if (clip) this.sourceSeconds = clip.start_seconds;
      this.musicNeedsSeek = true;
    }
    if (this.music?.url !== previousMusicURL) {
      this.musicNeedsSeek = true;
    }
    if (this.context) {
      this.updateMusicElement();
      this.applySync();
    }
  }

  sync(sourceSeconds: number, playing: boolean): void {
    if (this.disposed) return;
    this.sourceSeconds = Number.isFinite(sourceSeconds) ? sourceSeconds : 0;
    this.playing = playing;
    if (!this.context) return;
    this.applySync();
  }

  async resume(): Promise<void> {
    if (this.disposed) return;
    const generation = this.generation;
    try {
      this.ensureGraph();
      await this.context?.resume();
      if (this.disposed || generation !== this.generation) return;
      this.applySync();
    } catch (error) {
      if (this.disposed || generation !== this.generation) return;
      this.reportError();
      throw error;
    }
  }

  pause(): void {
    if (this.disposed) return;
    this.playing = false;
    this.pauseMusic();
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.generation += 1;
    this.playing = false;
    this.releaseMusicElement();
    this.sourceNode?.disconnect();
    this.sourceGain?.disconnect();
    this.sourceNode = null;
    this.sourceGain = null;
    const context = this.context;
    this.context = null;
    if (context) void context.close().catch(() => {});
  }

  private ensureGraph(): void {
    if (this.context || this.disposed) return;
    const context = new AudioContext();
    const sourceNode = context.createMediaElementSource(this.video);
    const sourceGain = context.createGain();
    sourceNode.connect(sourceGain);
    sourceGain.connect(context.destination);
    this.context = context;
    this.sourceNode = sourceNode;
    this.sourceGain = sourceGain;
    this.updateMusicElement();
  }

  private updateMusicElement(): void {
    const activeURL = this.clip ? this.music?.url : undefined;
    if (!activeURL) {
      this.releaseMusicElement();
      return;
    }
    if (this.musicElement && this.musicURL === activeURL) return;

    this.releaseMusicElement();
    if (!this.context) return;
    const element = new Audio();
    element.src = activeURL;
    element.loop = true;
    element.preload = 'auto';
    const generation = this.generation;
    element.onerror = () => {
      if (!this.disposed && generation === this.generation && this.musicElement === element) {
        this.reportError();
      }
    };
    const node = this.context.createMediaElementSource(element);
    const gain = this.context.createGain();
    node.connect(gain);
    gain.connect(this.context.destination);
    this.musicElement = element;
    this.musicURL = activeURL;
    this.musicNode = node;
    this.musicGain = gain;
    this.musicNeedsSeek = true;
  }

  private releaseMusicElement(): void {
    const element = this.musicElement;
    this.musicElement = null;
    this.musicURL = null;
    this.musicPlayToken += 1;
    this.musicPlayInFlight = false;
    if (element) {
      element.onerror = null;
      element.pause();
      element.removeAttribute('src');
      element.load();
    }
    this.musicNode?.disconnect();
    this.musicGain?.disconnect();
    this.musicNode = null;
    this.musicGain = null;
  }

  private applySync(): void {
    if (!this.context || !this.sourceGain) return;
    const timing = mixTiming(this.clip, this.sourceSeconds);
    this.sourceGain.gain.setValueAtTime(timing.sourceGain, this.context.currentTime);

    const element = this.musicElement;
    const musicGain = this.musicGain;
    if (!element || !musicGain || !this.clip || !this.music) return;
    musicGain.gain.setValueAtTime(timing.fade * effectiveMusicVolume(this.music.volume), this.context.currentTime);
    element.playbackRate = 1;

    const target = loopedMusicTime(timing.outputSeconds, element.duration);
    if (this.musicNeedsSeek || Math.abs(element.currentTime - target) > MUSIC_SEEK_DRIFT_SECONDS) {
      try {
        element.currentTime = target;
        this.musicNeedsSeek = false;
      } catch {
        // Metadata may not be ready yet; the next sync retries the bounded seek.
      }
    }
    if (this.playing) this.playMusic(element);
    else this.pauseMusic();
  }

  private playMusic(element: HTMLAudioElement): void {
    if (!element.paused || this.musicPlayInFlight) return;
    const generation = this.generation;
    const token = ++this.musicPlayToken;
    this.musicPlayInFlight = true;
    void element.play().then(() => {
      if (!this.disposed && generation === this.generation && token === this.musicPlayToken) {
        this.musicPlayInFlight = false;
      }
    }).catch(() => {
      if (!this.disposed && generation === this.generation && token === this.musicPlayToken && this.musicElement === element) {
        this.musicPlayInFlight = false;
        this.reportError();
      }
    });
  }

  private pauseMusic(): void {
    this.musicPlayToken += 1;
    this.musicPlayInFlight = false;
    this.musicElement?.pause();
  }

  private reportError(): void {
    try {
      this.onError();
    } catch {
      // A consumer error callback must not break playback cleanup.
    }
  }
}

function mixTiming(clip: StreamClipRange | null, sourceSeconds: number): {
  fade: number;
  outputSeconds: number;
  sourceGain: number;
} {
  if (!clip) return { fade: 1, outputSeconds: 0, sourceGain: 1 };
  const speed = finiteBetween(clip.edit?.speed, 0.25, 3, 1);
  const sourceDuration = Math.max(0, clip.end_seconds - clip.start_seconds);
  const outputDuration = sourceDuration / speed;
  const outputSeconds = clamp((sourceSeconds - clip.start_seconds) / speed, 0, outputDuration);
  const fadeIn = finiteBetween(clip.edit?.fade_in_seconds, 0, 5, 0);
  const fadeOut = finiteBetween(clip.edit?.fade_out_seconds, 0, 5, 0);
  const fade = streamFadeEnvelope(outputSeconds, outputDuration, fadeIn, fadeOut);
  const sourceVolume = finiteBetween(clip.edit?.source_volume, 0, 2, 1);
  return { fade, outputSeconds, sourceGain: sourceVolume * fade };
}

function effectiveMusicVolume(volume: number): number {
  if (volume === 0) return DEFAULT_MUSIC_VOLUME;
  return finiteBetween(volume, 0, 1, DEFAULT_MUSIC_VOLUME);
}

function loopedMusicTime(outputSeconds: number, duration: number): number {
  return Number.isFinite(duration) && duration > 0 ? outputSeconds % duration : outputSeconds;
}

function clipIdentity(clip: StreamClipRange | null): string | null {
  return clip
    ? `${clip.id}\u0000${clip.start_seconds}\u0000${clip.end_seconds}\u0000${clip.edit?.speed ?? 1}`
    : null;
}

function validMusic(music: MusicSelection | null): MusicSelection | null {
  if (!music || typeof music.url !== 'string' || music.url.trim() === '') return null;
  return { url: music.url, volume: music.volume };
}

function finiteBetween(value: number | undefined, min: number, max: number, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value)
    ? clamp(value, min, max)
    : fallback;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
