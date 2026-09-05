'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Expand,
  Maximize2,
  Minimize2,
  Pause,
  Play,
  Repeat2,
  Volume2,
  VolumeX,
} from 'lucide-react';
import { PLAYBACK_REVIEW, type MediaPlaybackItem } from '@/lib/api/playback';
import { claimMediaPlayback } from '@/lib/media-playback-owner';
import {
  MEDIA_PLAYBACK_STORAGE_KEY,
  mediaPlaybackKey,
  parseMediaPlaybackStore,
  updateMediaPlaybackStore,
} from '@/lib/media-playback-store';
import { cn } from '@/lib/utils';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

const SEEK_SECONDS = 5;
const WRITE_INTERVAL_MS = 5000;
const MIN_END_GUARD_SECONDS = 0.5;
const PLAYBACK_RATES = [0.25, 0.5, 1, 1.5, 2, 3] as const;
type MediaStatus = 'loading' | 'ready' | 'buffering' | 'seeking';

export type MediaPlayerProps = {
  items: readonly MediaPlaybackItem[];
  activeId: string | null;
  open: boolean;
  onActiveChange?: (id: string) => void;
  onOpenChange: (open: boolean) => void;
  returnFocus?: HTMLElement | null;
};

function clock(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00';
  const whole = Math.floor(seconds);
  const minutes = Math.floor(whole / 60);
  return `${minutes}:${String(whole % 60).padStart(2, '0')}`;
}

/** Shared Chromium player for immutable demo and stream render artifacts. */
export function MediaPlayer({
  items,
  activeId,
  open,
  onActiveChange,
  onOpenChange,
  returnFocus,
}: MediaPlayerProps): ReactNode {
  const videoRef = useRef<HTMLVideoElement>(null);
  const playButtonRef = useRef<HTMLButtonElement>(null);
  const fullscreenRef = useRef<HTMLDivElement>(null);
  const owner = useRef<object>({});
  const releaseOwnership = useRef<(() => void) | null>(null);
  const playbackGeneration = useRef(0);
  const lastWrite = useRef(0);
  const [playing, setPlaying] = useState(false);
  const [duration, setDuration] = useState(0);
  const [position, setPosition] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [loop, setLoop] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [mediaStatus, setMediaStatus] = useState<MediaStatus>('loading');
  const [fullscreen, setFullscreen] = useState(false);
  const [fullscreenError, setFullscreenError] = useState(false);
  const [error, setError] = useState(false);

  const activeIndex = useMemo(() => items.findIndex((item) => item.id === activeId), [activeId, items]);
  const item = activeIndex >= 0 ? items[activeIndex] : undefined;
  const storageKey = item ? mediaPlaybackKey(item) : null;
  const playbackUrl = item?.playbackUrl;

  const persistVideo = useCallback((video: HTMLVideoElement, key: string): void => {
    try {
      const raw = window.localStorage.getItem(MEDIA_PLAYBACK_STORAGE_KEY);
      window.localStorage.setItem(
        MEDIA_PLAYBACK_STORAGE_KEY,
        updateMediaPlaybackStore(raw, key, {
          position: video.currentTime,
          volume: video.volume,
          muted: video.muted,
          loop: video.loop,
          playbackRate: video.playbackRate,
          updatedAt: Date.now(),
        }),
      );
      lastWrite.current = Date.now();
    } catch {
      // Playback remains available when storage is disabled or full.
    }
  }, []);

  const persist = useCallback((): void => {
    const video = videoRef.current;
    if (video !== null && storageKey !== null) persistVideo(video, storageKey);
  }, [persistVideo, storageKey]);

  const pause = useCallback((): void => {
    playbackGeneration.current += 1;
    videoRef.current?.pause();
  }, []);

  const release = useCallback((): void => {
    releaseOwnership.current?.();
    releaseOwnership.current = null;
  }, []);

  const togglePlayback = useCallback(async (): Promise<void> => {
    const video = videoRef.current;
    if (video === null) return;
    if (!video.paused) {
      video.pause();
      return;
    }
    const generation = playbackGeneration.current + 1;
    playbackGeneration.current = generation;
    releaseOwnership.current = claimMediaPlayback(owner.current, pause);
    try {
      await video.play();
      if (playbackGeneration.current !== generation || videoRef.current !== video || !open) video.pause();
    } catch {
      release();
      if (playbackGeneration.current === generation && videoRef.current === video && open) setError(true);
    }
  }, [open, pause, release]);

  const move = useCallback(
    (step: -1 | 1): void => {
      const next = activeIndex + step;
      const candidate = items[next];
      if (candidate === undefined) return;
      persist();
      pause();
      release();
      onActiveChange?.(candidate.id);
    },
    [activeIndex, items, onActiveChange, pause, persist, release],
  );

  const toggleFullscreen = useCallback(async (): Promise<void> => {
    const frame = fullscreenRef.current;
    if (frame === null) return;
    try {
      if (document.fullscreenElement === frame) await document.exitFullscreen();
      else await frame.requestFullscreen();
      setFullscreenError(false);
    } catch {
      setFullscreenError(true);
    }
  }, []);

  useEffect(() => {
    const changed = (): void => setFullscreen(document.fullscreenElement === fullscreenRef.current);
    document.addEventListener('fullscreenchange', changed);
    return () => document.removeEventListener('fullscreenchange', changed);
  }, []);

  useEffect(() => {
    if (!open) {
      if (returnFocus?.isConnected) returnFocus.focus();
      return;
    }
    const video = videoRef.current;
    const key = storageKey;
    if (video !== null && playbackUrl !== undefined && video.getAttribute('src') !== playbackUrl) {
      video.src = playbackUrl;
      video.load();
    }
    return () => {
      playbackGeneration.current += 1;
      if (video !== null && key !== null) persistVideo(video, key);
      video?.pause();
      video?.removeAttribute('src');
      video?.load();
      release();
    };
  }, [open, persistVideo, playbackUrl, release, returnFocus, storageKey]);

  useEffect(() => {
    if (!open || item !== undefined) return;
    pause();
    release();
    onOpenChange(false);
    if (returnFocus?.isConnected) returnFocus.focus();
  }, [item, onOpenChange, open, pause, release, returnFocus]);

  useEffect(() => {
    if (!open) return;
    const suspend = (): void => {
      persist();
      pause();
    };
    const onVisibilityChange = (): void => {
      if (document.visibilityState === 'hidden') suspend();
    };
    document.addEventListener('visibilitychange', onVisibilityChange);
    window.addEventListener('pagehide', suspend);
    return () => {
      document.removeEventListener('visibilitychange', onVisibilityChange);
      window.removeEventListener('pagehide', suspend);
    };
  }, [open, pause, persist]);

  useEffect(() => {
    setPlaying(false);
    setPosition(0);
    setDuration(0);
    setError(false);
    setFullscreenError(false);
    setMediaStatus('loading');
    lastWrite.current = Date.now();
    if (storageKey === null) return;
    try {
      const saved = parseMediaPlaybackStore(window.localStorage.getItem(MEDIA_PLAYBACK_STORAGE_KEY)).entries[storageKey];
      setVolume(saved?.volume ?? 1);
      setMuted(saved?.muted ?? false);
      setLoop(saved?.loop ?? false);
      setPlaybackRate(saved?.playbackRate ?? 1);
    } catch {
      setVolume(1);
      setMuted(false);
      setLoop(false);
      setPlaybackRate(1);
    }
  }, [storageKey]);

  useEffect(() => {
    if (videoRef.current !== null) {
      videoRef.current.volume = volume;
      videoRef.current.playbackRate = playbackRate;
    }
  }, [playbackRate, storageKey, volume]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.target instanceof HTMLInputElement) return;
      if (event.target instanceof HTMLButtonElement && (event.key === ' ' || event.key === 'Enter')) return;
      const video = videoRef.current;
      if (video === null) return;
      switch (event.key.toLowerCase()) {
        case ' ':
          event.preventDefault();
          void togglePlayback();
          break;
        case 'arrowleft':
          event.preventDefault();
          video.currentTime = Math.max(0, video.currentTime - SEEK_SECONDS);
          break;
        case 'arrowright':
          event.preventDefault();
          video.currentTime = Math.min(video.duration || 0, video.currentTime + SEEK_SECONDS);
          break;
        case 'm':
          setMuted((value) => !value);
          break;
        case 'l':
          setLoop((value) => !value);
          break;
        case 'f':
          void toggleFullscreen();
          break;
        case '[':
          move(-1);
          break;
        case ']':
          move(1);
          break;
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [move, open, toggleFullscreen, togglePlayback]);

  if (item === undefined) return null;

  let reviewLabel: string | null = null;
  if (item.review === PLAYBACK_REVIEW.pending) reviewLabel = 'Revisión QA pendiente';
  else if (item.review === PLAYBACK_REVIEW.stale) reviewLabel = 'Render desactualizado';
  let mediaStatusLabel = '';
  if (mediaStatus === 'loading') mediaStatusLabel = 'Cargando';
  else if (mediaStatus === 'buffering') mediaStatusLabel = 'Buffering';
  else if (mediaStatus === 'seeking') mediaStatusLabel = 'Buscando';

  return (
    <Dialog open={open} onOpenChange={(next) => {
      if (!next) persist();
      onOpenChange(next);
    }}>
      <DialogContent
        className="max-h-[calc(100dvh-1rem)] max-w-[min(92rem,calc(100%-1rem))] overflow-auto p-3 sm:p-4"
        onOpenAutoFocus={(event) => {
          event.preventDefault();
          playButtonRef.current?.focus({ preventScroll: true });
        }}
        onEscapeKeyDown={(event) => {
          if (fullscreen || document.fullscreenElement === fullscreenRef.current) {
            event.preventDefault();
            void document.exitFullscreen();
          }
        }}
      >
        <DialogHeader className="pr-12">
          <span className="flex flex-wrap items-center gap-2">
            <DialogTitle className="truncate">{item.title}</DialogTitle>
            <StatusTag tone={item.review === PLAYBACK_REVIEW.ready ? 'success' : 'warning'}>
              {item.format}
            </StatusTag>
          </span>
          <DialogDescription>
            {reviewLabel ?? 'Resultado renderizado'} · {activeIndex + 1} de {items.length}
          </DialogDescription>
        </DialogHeader>

        {reviewLabel !== null ? (
          <div className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning" role="status">
            <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
            <span>
              {reviewLabel}. La reproducción sirve para inspeccionarlo y no cambia su estado ni habilita acciones bloqueadas.
            </span>
          </div>
        ) : null}

        <div
          ref={fullscreenRef}
          className="flex min-h-0 flex-col overflow-hidden border border-border bg-surface-0 shadow-[var(--elev-3)]"
        >
          <div className="flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-black">
            {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
            <video
              key={storageKey}
              ref={videoRef}
              src={item.playbackUrl}
              poster={item.posterUrl}
              preload="metadata"
              playsInline
              muted={muted}
              loop={loop}
              className={cn(
                'max-h-[calc(100dvh-20rem)] w-full object-contain',
                item.format === '9:16' ? 'aspect-[9/16]' : 'aspect-video',
                fullscreen && 'max-h-[calc(100dvh-5rem)]',
              )}
              onClick={() => void togglePlayback()}
              onLoadedMetadata={(event) => {
                const video = event.currentTarget;
                setDuration(video.duration);
                if (storageKey === null) return;
                try {
                  const saved = parseMediaPlaybackStore(window.localStorage.getItem(MEDIA_PLAYBACK_STORAGE_KEY)).entries[storageKey];
                  video.volume = saved?.volume ?? 1;
                  video.muted = saved?.muted ?? false;
                  video.loop = saved?.loop ?? false;
                  video.playbackRate = saved?.playbackRate ?? 1;
                  const endGuard = Math.min(SEEK_SECONDS, Math.max(MIN_END_GUARD_SECONDS, video.duration * 0.05));
                  if (saved && saved.position < video.duration - endGuard) {
                    video.currentTime = saved.position;
                    setPosition(saved.position);
                  }
                } catch {
                  // Start at zero when persisted data cannot be read.
                }
              }}
              onLoadedData={() => setMediaStatus('ready')}
              onCanPlay={() => setMediaStatus('ready')}
              onWaiting={() => setMediaStatus('buffering')}
              onStalled={() => setMediaStatus('buffering')}
              onSeeking={() => setMediaStatus('seeking')}
              onSeeked={() => setMediaStatus('ready')}
              onPlay={() => {
                releaseOwnership.current = claimMediaPlayback(owner.current, pause);
                setPlaying(true);
                setMediaStatus('ready');
              }}
              onPause={(event) => {
                setPlaying(false);
                if (storageKey !== null) persistVideo(event.currentTarget, storageKey);
                release();
              }}
              onEnded={(event) => {
                setPlaying(false);
                if (storageKey !== null) persistVideo(event.currentTarget, storageKey);
                release();
              }}
              onTimeUpdate={(event) => {
                setPosition(event.currentTarget.currentTime);
                if (storageKey !== null && Date.now() - lastWrite.current >= WRITE_INTERVAL_MS) {
                  persistVideo(event.currentTarget, storageKey);
                }
              }}
              onError={(event) => {
                if (videoRef.current !== event.currentTarget) return;
                event.currentTarget.pause();
                release();
                setPlaying(false);
                setError(true);
                setMediaStatus('loading');
              }}
            />
          </div>

          <div className="flex flex-col gap-2 border-t border-border bg-surface-3 p-2.5">
            <label className="sr-only" htmlFor="media-player-position">Posición</label>
            <input
              id="media-player-position"
              type="range"
              min={0}
              max={duration || 0}
              step={0.05}
              value={Math.min(position, duration || 0)}
              onChange={(event) => {
                const next = Number(event.currentTarget.value);
                if (videoRef.current !== null) videoRef.current.currentTime = next;
                setPosition(next);
              }}
              className="h-10 w-full accent-primary"
            />
            <div className="flex flex-wrap items-center gap-1.5">
              <Button ref={playButtonRef} variant="outline-primary" size="icon-sm" onClick={() => void togglePlayback()} aria-label={playing ? 'Pausar' : 'Reproducir'}>
                {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
              </Button>
              <span className="min-w-24 font-mono text-meta tabular-nums text-fg-2">{clock(position)} / {clock(duration)}</span>
              <span className="font-mono text-meta uppercase tracking-wider text-fg-3" role="status">
                {mediaStatusLabel}
              </span>
              <Button variant="ghost" size="icon-sm" onClick={() => setMuted((value) => !value)} aria-label={muted ? 'Activar sonido' : 'Silenciar'}>
                {muted ? <VolumeX aria-hidden /> : <Volume2 aria-hidden />}
              </Button>
              <label className="sr-only" htmlFor="media-player-volume">Volumen</label>
              <input
                id="media-player-volume"
                type="range"
                min={0}
                max={1}
                step={0.05}
                value={volume}
                onChange={(event) => setVolume(Number(event.currentTarget.value))}
                className="h-10 w-20 accent-primary sm:w-28"
              />
              <Button variant={loop ? 'outline-primary' : 'ghost'} size="icon-sm" onClick={() => setLoop((value) => !value)} aria-pressed={loop} aria-label="Repetir vídeo">
                <Repeat2 aria-hidden />
              </Button>
              <label className="flex h-10 items-center gap-1.5 font-mono text-meta uppercase tracking-wider text-fg-3">
                Velocidad
                <select
                  value={playbackRate}
                  onChange={(event) => setPlaybackRate(Number(event.currentTarget.value))}
                  className="h-10 border border-border-strong bg-surface-2 px-2 text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                >
                  {PLAYBACK_RATES.map((rate) => <option key={rate} value={rate}>{rate}×</option>)}
                </select>
              </label>
              <span className="ml-auto flex items-center gap-1.5">
                <Button variant="ghost" size="icon-sm" disabled={onActiveChange === undefined || activeIndex <= 0} onClick={() => move(-1)} aria-label="Vídeo anterior">
                  <ChevronLeft aria-hidden />
                </Button>
                <Button variant="ghost" size="icon-sm" disabled={onActiveChange === undefined || activeIndex >= items.length - 1} onClick={() => move(1)} aria-label="Vídeo siguiente">
                  <ChevronRight aria-hidden />
                </Button>
                <Button variant="ghost" size="icon-sm" onClick={() => void toggleFullscreen()} aria-label={fullscreen ? 'Salir de pantalla completa' : 'Pantalla completa'}>
                  {fullscreen ? <Minimize2 aria-hidden /> : <Maximize2 aria-hidden />}
                </Button>
              </span>
            </div>
            {error ? (
              <div className="flex flex-wrap items-center gap-2 text-body-sm text-destructive" role="alert">
                <Expand aria-hidden className="size-4" />
                <span>No se pudo reproducir este archivo. Comprueba que sigue disponible y que Chromium admite su códec.</span>
                <Button
                  type="button"
                  size="xs"
                  variant="outline"
                  onClick={() => {
                    const video = videoRef.current;
                    if (video === null) return;
                    setError(false);
                    setMediaStatus('loading');
                    video.load();
                  }}
                >
                  Reintentar reproducción
                </Button>
              </div>
            ) : null}
            {fullscreenError ? (
              <p className="text-body-sm text-warning" role="status">No se pudo abrir la pantalla completa.</p>
            ) : null}
          </div>
        </div>

        {item.warnings.length > 0 ? (
          <ul className="flex list-disc flex-col gap-1 pl-5 text-body-sm text-warning">
            {item.warnings.map((warning) => <li key={warning}>{warning}</li>)}
          </ul>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
