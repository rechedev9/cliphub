import { LatestFrameRequest } from './stream-frame-session.ts';

export const PLAYBACK_STATUS = {
  loading: 'loading', ready: 'ready', playing: 'playing', paused: 'paused',
  seeking: 'seeking', buffering: 'buffering', ended: 'ended', error: 'error',
} as const;

export type PlaybackStatus = (typeof PLAYBACK_STATUS)[keyof typeof PLAYBACK_STATUS];
export type PlaybackFrame = {
  seconds: number;
  width: number;
  height: number;
  presentedFrames: number;
};
export type PlaybackState = {
  status: PlaybackStatus;
  seconds: number;
  duration: number;
  requestedSeconds: number | null;
  playRequested: boolean;
};
export type PlaybackRange = { start: number; end: number; rate: number };
type PlaybackVideo = Pick<HTMLVideoElement,
  'currentTime' | 'duration' | 'paused' | 'ended' | 'readyState' | 'playbackRate'
  | 'videoWidth' | 'videoHeight' | 'play' | 'pause' | 'addEventListener' | 'removeEventListener'
  | 'requestVideoFrameCallback' | 'cancelVideoFrameCallback'>;

export class PlaybackSession {
  #video: PlaybackVideo;
  #requests = new LatestFrameRequest();
  #frameListeners = new Set<(frame: PlaybackFrame) => void>();
  #listeners: Array<[string, EventListener]> = [];
  #onState: (state: PlaybackState) => void;
  #onRangeEnd: () => void;
  #callback: number | null = null;
  #range: PlaybackRange | null = null;
  #disposed = false;
  #wanted = false;
  #seeking = false;
  #generation = 0;
  #playInFlight = false;
  #requestedSeconds: number | null = null;
  #status: PlaybackStatus = PLAYBACK_STATUS.loading;
  #lastPositionNotification = -Infinity;
  #frame: PlaybackFrame;

  constructor(video: PlaybackVideo, options: {
    onState: (state: PlaybackState) => void;
    onRangeEnd?: () => void;
  }) {
    this.#video = video;
    this.#onState = options.onState;
    this.#onRangeEnd = options.onRangeEnd ?? (() => {});
    this.#frame = { seconds: 0, width: video.videoWidth, height: video.videoHeight, presentedFrames: 0 };
    this.#listen('loadedmetadata', () => { this.#notify(PLAYBACK_STATUS.ready); this.#seekNext(); });
    this.#listen('loadeddata', () => { this.redraw(); this.#tryPlay(); });
    this.#listen('seeked', () => this.#settled());
    this.#listen('playing', () => {
      if (!this.#wanted || this.#seeking) { this.#video.pause(); return; }
      this.#notify(PLAYBACK_STATUS.playing);
      this.#scheduleFrame();
    });
    this.#listen('pause', () => {
      if (this.#seeking || this.#disposed || this.#playInFlight) return;
      this.#wanted = false;
      this.#cancelFrame();
      this.#notify(PLAYBACK_STATUS.paused);
    });
    this.#listen('waiting', () => { if (this.#wanted) this.#notify(PLAYBACK_STATUS.buffering); });
    this.#listen('ended', () => this.#endRange());
    this.#listen('error', () => this.#fail());
    this.#listen('timeupdate', () => {
      if (this.#wanted && this.#range && video.currentTime >= this.#range.end) this.#endRange();
    });
    if (video.readyState >= 2) this.redraw();
  }

  get frame(): PlaybackFrame { return this.#frame; }
  get playRequested(): boolean { return this.#wanted; }

  subscribeFrames(listener: (frame: PlaybackFrame) => void): () => void {
    this.#frameListeners.add(listener);
    return () => { this.#frameListeners.delete(listener); };
  }

  setRange(range: PlaybackRange | null): void {
    this.#range = range;
    const rate = range?.rate ?? 1;
    this.#video.playbackRate = Number.isFinite(rate) && rate > 0 ? rate : 1;
  }

  play(): void {
    if (this.#disposed || this.#wanted) return;
    this.#wanted = true;
    if (this.#range && (this.#video.currentTime < this.#range.start || this.#video.currentTime >= this.#range.end)) {
      this.seek(this.#range.start);
    }
    this.#tryPlay();
  }

  pause(): void {
    if (this.#disposed) return;
    this.#generation += 1;
    this.#wanted = false;
    this.#playInFlight = false;
    this.#video.pause();
    this.#cancelFrame();
    this.#notify(PLAYBACK_STATUS.paused);
  }

  seek(seconds: number): void {
    if (this.#disposed || !Number.isFinite(seconds)) return;
    this.#generation += 1;
    this.#playInFlight = false;
    const target = this.#wanted && this.#range
      ? Math.max(this.#range.start, Math.min(seconds, this.#range.end - Math.min(0.01, (this.#range.end - this.#range.start) / 2)))
      : seconds;
    this.#requests.request(target);
    this.#requestedSeconds = target;
    if (this.#seeking) { this.#notify(PLAYBACK_STATUS.seeking); return; }
    this.#seekNext();
  }

  redraw(): void {
    if (this.#disposed || this.#seeking || this.#video.readyState < 2) return;
    this.#frame = { ...this.#frame, seconds: this.#video.currentTime, width: this.#video.videoWidth, height: this.#video.videoHeight };
    this.#emitFrame();
    this.#notify(this.#wanted ? this.#status : PLAYBACK_STATUS.paused);
  }

  dispose(): void {
    if (this.#disposed) return;
    this.#disposed = true;
    this.#wanted = false;
    this.#generation += 1;
    this.#cancelFrame();
    for (const [name, listener] of this.#listeners) this.#video.removeEventListener(name, listener);
    this.#listeners = [];
    this.#frameListeners.clear();
    this.#video.pause();
  }

  #listen(name: string, listener: EventListener): void {
    this.#listeners.push([name, listener]);
    this.#video.addEventListener(name, listener);
  }

  #seekNext(): void {
    if (this.#video.readyState < 1 || this.#disposed) return;
    const next = this.#requests.next(this.#video.currentTime, this.#video.duration);
    if (next === null) { this.#requestedSeconds = null; this.redraw(); this.#tryPlay(); return; }
    this.#seeking = true;
    this.#video.pause();
    this.#cancelFrame();
    this.#notify(PLAYBACK_STATUS.seeking);
    this.#video.currentTime = next;
    this.#scheduleFrame();
  }

  #settled(): void {
    if (this.#disposed) return;
    this.#seeking = false;
    const next = this.#requests.settled(this.#video.currentTime, this.#video.duration);
    if (next !== null) {
      this.#seeking = true;
      this.#video.currentTime = next;
      this.#scheduleFrame();
      return;
    }
    this.#requestedSeconds = null;
    this.redraw();
    this.#tryPlay();
  }

  #tryPlay(): void {
    if (this.#disposed || !this.#wanted || this.#seeking || this.#video.readyState < 2 || this.#playInFlight) return;
    const generation = this.#generation;
    this.#playInFlight = true;
    void this.#video.play().then(() => {
      if (this.#disposed || !this.#wanted) { this.#video.pause(); return; }
      if (generation !== this.#generation) return;
      this.#playInFlight = false;
      this.#notify(PLAYBACK_STATUS.playing);
      this.#scheduleFrame();
    }).catch(() => {
      if (this.#disposed || generation !== this.#generation) return;
      this.#playInFlight = false;
      this.#fail();
    });
  }

  #scheduleFrame(): void {
    if (this.#callback !== null || this.#disposed) return;
    this.#callback = this.#video.requestVideoFrameCallback((now, metadata) => {
      this.#callback = null;
      if (this.#disposed) return;
      if (!this.#seeking) {
        if (this.#wanted && this.#range && metadata.mediaTime >= this.#range.end) {
          this.#endRange();
          return;
        }
        this.#frame = { seconds: metadata.mediaTime, width: this.#video.videoWidth, height: this.#video.videoHeight, presentedFrames: metadata.presentedFrames };
        this.#emitFrame();
        if (now - this.#lastPositionNotification >= 100) {
          this.#lastPositionNotification = now;
          this.#notify(this.#wanted ? this.#status : PLAYBACK_STATUS.paused);
        }
      }
      if (this.#wanted || this.#seeking) this.#scheduleFrame();
    });
  }

  #emitFrame(): void {
    for (const listener of this.#frameListeners) listener(this.#frame);
  }

  #cancelFrame(): void {
    if (this.#callback !== null) this.#video.cancelVideoFrameCallback(this.#callback);
    this.#callback = null;
  }

  #endRange(): void {
    if (!this.#wanted || this.#disposed || this.#seeking) return;
    this.pause();
    this.#notify(PLAYBACK_STATUS.ended);
    this.#onRangeEnd();
  }

  #fail(): void {
    this.pause();
    this.#notify(PLAYBACK_STATUS.error);
  }

  #notify(status: PlaybackStatus): void {
    if (this.#disposed) return;
    this.#status = status;
    this.#onState({ status, seconds: this.#frame.seconds, requestedSeconds: this.#requestedSeconds, duration: Number.isFinite(this.#video.duration) ? this.#video.duration : 0, playRequested: this.#wanted });
  }
}
