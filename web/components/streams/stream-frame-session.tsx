'use client';

import { createContext, useContext, useEffect, useRef, useState, type ReactElement, type ReactNode } from 'react';
import type { NormalizedRect, StreamClipRange } from '@/lib/api/streams';
import { browserWindowActivity } from '@/lib/window-activity';
import { claimMediaPlayback } from '@/lib/media-playback-owner';
import { PlaybackSession, PLAYBACK_STATUS, type PlaybackStatus } from '@/lib/playback-session';
import { StreamAudioMixer } from '@/lib/stream-audio';
import { nextStreamPlaybackIndex, streamClipRange, streamPlaybackIndex, STREAM_PLAYBACK_MODE, type StreamPlaybackMode } from '@/lib/stream-playback';

const MAX_CANVAS_WIDTH = 1440;
type StreamFrameState = {
  sourceHeight: number;
  sourceWidth: number;
  video: HTMLVideoElement | null;
  session: PlaybackSession | null;
};
type StreamFrameProps = {
  children: ReactNode;
  seek: { seconds: number; revision: number };
  playing: boolean;
  mode: StreamPlaybackMode;
  loop: boolean;
  clips: StreamClipRange[];
  selectedClipId: string | null;
  music: { url: string; volume: number } | null;
  onPosition: (seconds: number) => void;
  onStatus: (status: PlaybackStatus) => void;
  onPlayingChange: (playing: boolean) => void;
  onClipChange: (clipId: string | null) => void;
  onMediaError: () => void;
  videoSrc: string;
};
type StreamRuntime = {
  play: () => void;
  pause: () => void;
  seek: (seconds: number) => void;
  configure: () => void;
};
const StreamFrameContext = createContext<StreamFrameState | null>(null);

export function StreamFrameSession(props: StreamFrameProps): ReactElement {
  const hostRef = useRef<HTMLDivElement>(null);
  const latest = useRef(props);
  latest.current = props;
  const runtimeRef = useRef<StreamRuntime | null>(null);
  const [frame, setFrame] = useState<StreamFrameState>({ sourceHeight: 0, sourceWidth: 0, video: null, session: null });

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const video = document.createElement('video');
    video.preload = 'metadata';
    video.playsInline = true;
    video.width = 1;
    video.height = 1;
    video.dataset.streamFrame = 'shared-decoder';
    video.setAttribute('aria-hidden', 'true');
    let alive = true;
    let playGeneration = 0;
    let clipIndex = -1;
    let playbackClipId: string | null = null;
    let videoPlaying = false;
    let rangeKey = '';
    let selectionKey = '';
    let releaseOwner = (): void => {};
    const mixer = new StreamAudioMixer(video, () => { if (alive) latest.current.onMediaError(); });
    const session = new PlaybackSession(video, {
      onState: (state) => {
        if (!alive) return;
        latest.current.onPosition(state.requestedSeconds ?? state.seconds);
        latest.current.onStatus(state.status);
        videoPlaying = state.status === PLAYBACK_STATUS.playing;
        mixer.sync(state.seconds, videoPlaying);
        if (state.status === PLAYBACK_STATUS.error) {
          mixer.pause();
          latest.current.onPlayingChange(false);
          latest.current.onMediaError();
        } else if (state.status === PLAYBACK_STATUS.paused && !state.playRequested && latest.current.playing) {
          queueMicrotask(() => {
            if (!alive || session.playRequested || !latest.current.playing) return;
            latest.current.onPlayingChange(false);
          });
        }
      },
      onRangeEnd: () => {
        const current = latest.current;
        if (current.mode === STREAM_PLAYBACK_MODE.source && current.loop) {
          session.seek(0);
          session.play();
          return;
        }
        clipIndex = nextStreamPlaybackIndex(current.clips, clipIndex, current.mode, current.loop);
        if (clipIndex < 0) { runtime.pause(); latest.current.onPlayingChange(false); return; }
        applyClip();
        const clip = current.clips[clipIndex];
        if (clip) session.seek(clip.start_seconds);
        session.play();
      },
    });

    function applyClip(): void {
      const current = latest.current;
      const clip = current.mode === STREAM_PLAYBACK_MODE.source ? undefined : current.clips[clipIndex];
      playbackClipId = clip?.id ?? null;
      session.setRange(streamClipRange(clip));
      mixer.configure(clip ?? null, clip ? current.music : null);
      latest.current.onClipChange(clip?.id ?? null);
    }

    const runtime: StreamRuntime = {
      configure: () => {
        const current = latest.current;
        const nextSelection = current.mode + ':' + (current.selectedClipId ?? '');
        const requestedId = current.mode === STREAM_PLAYBACK_MODE.selected || nextSelection !== selectionKey
          ? current.selectedClipId : playbackClipId;
        clipIndex = streamPlaybackIndex(current.clips, requestedId);
        selectionKey = nextSelection;
        const clip = current.mode === STREAM_PLAYBACK_MODE.source ? undefined : current.clips[clipIndex];
        const nextRange = JSON.stringify([current.mode, clip?.id, streamClipRange(clip)]);
        const changed = rangeKey !== '' && rangeKey !== nextRange;
        rangeKey = nextRange;
        applyClip();
        if (changed && clip && (video.currentTime < clip.start_seconds || video.currentTime >= clip.end_seconds)) session.seek(clip.start_seconds);
        if (current.mode !== STREAM_PLAYBACK_MODE.source && !clip) {
          runtime.pause();
          current.onPlayingChange(false);
        }
      },
      play: () => {
        if (!alive || !browserWindowActivity.isActive()) { latest.current.onPlayingChange(false); return; }
        const generation = ++playGeneration;
        releaseOwner = claimMediaPlayback(session, () => { runtime.pause(); latest.current.onPlayingChange(false); });
        void mixer.resume().then(() => {
          if (alive && generation === playGeneration && latest.current.playing) session.play();
        }).catch(() => {
          if (!alive || generation !== playGeneration) return;
          runtime.pause();
          latest.current.onPlayingChange(false);
          latest.current.onMediaError();
        });
      },
      pause: () => {
        playGeneration += 1;
        session.pause();
        mixer.pause();
        releaseOwner();
      },
      seek: (seconds) => { session.seek(seconds); mixer.sync(seconds, false); },
    };
    runtimeRef.current = runtime;
    const unsubscribeFrames = session.subscribeFrames((next) => mixer.sync(next.seconds, videoPlaying && session.playRequested && !video.paused));
    const onMetadata = (): void => {
      if (!alive) return;
      setFrame({ video, session, sourceWidth: video.videoWidth, sourceHeight: video.videoHeight });
    };
    video.addEventListener('loadedmetadata', onMetadata);
    const unsubscribeActivity = browserWindowActivity.subscribe(() => {
      if (!browserWindowActivity.isActive()) { runtime.pause(); latest.current.onPlayingChange(false); }
    });
    host.append(video);
    runtime.configure();
    session.seek(latest.current.seek.seconds);
    video.src = props.videoSrc;
    video.load();
    if (latest.current.playing) runtime.play();
    return () => {
      alive = false;
      playGeneration += 1;
      runtimeRef.current = null;
      releaseOwner();
      unsubscribeActivity();
      unsubscribeFrames();
      video.removeEventListener('loadedmetadata', onMetadata);
      session.dispose();
      mixer.dispose();
      video.removeAttribute('src');
      video.load();
      video.remove();
    };
  }, [props.videoSrc]);

  useEffect(() => { runtimeRef.current?.configure(); }, [props.mode, props.selectedClipId, props.clips, props.music]);
  useEffect(() => { runtimeRef.current?.seek(props.seek.seconds); }, [props.seek]);
  useEffect(() => {
    if (props.playing) runtimeRef.current?.play();
    else runtimeRef.current?.pause();
  }, [props.playing]);

  return (
    <StreamFrameContext.Provider value={frame}>
      <div ref={hostRef} className="pointer-events-none absolute h-px w-px overflow-hidden opacity-0" aria-hidden />
      {props.children}
    </StreamFrameContext.Provider>
  );
}

export function useStreamFrame(): StreamFrameState {
  const state = useContext(StreamFrameContext);
  if (state === null) throw new Error('StreamFrameCanvas must be rendered inside StreamFrameSession');
  return state;
}

export function StreamFrameCanvas({ className, mode, outputHeight, outputWidth, rect }: {
  className?: string;
  mode: 'contain' | 'cover' | 'stretch';
  outputHeight?: number;
  outputWidth?: number;
  rect?: NormalizedRect;
}): ReactElement {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const frame = useStreamFrame();
  const geometry = useRef({ mode, outputHeight, outputWidth, rect });
  geometry.current = { mode, outputHeight, outputWidth, rect };
  const drawRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const { video, session } = frame;
    if (!canvas || !video || !session) return;
    const context = canvas.getContext('2d', { alpha: false });
    if (!context) return;
    let visible = false;
    const draw = (): void => {
      if (!visible || video.readyState < 2 || video.videoWidth <= 0 || video.videoHeight <= 0) return;
      const { mode: fit, outputHeight: oh, outputWidth: ow, rect: cropRect } = geometry.current;
      const crop = cropRect ?? { x: 0, y: 0, width: 1, height: 1 };
      let sx = crop.x * video.videoWidth;
      let sy = crop.y * video.videoHeight;
      let sw = crop.width * video.videoWidth;
      let sh = crop.height * video.videoHeight;
      let dw = canvas.width;
      let dh = canvas.height;
      if (fit === 'cover') {
        const aspect = (ow ?? dw) / (oh ?? dh);
        if (sw / sh > aspect) { const width = sh * aspect; sx += (sw - width) / 2; sw = width; }
        else { const height = sw / aspect; sy += (sh - height) / 2; sh = height; }
      } else if (fit === 'contain') {
        const scale = Math.min(dw / sw, dh / sh);
        dw = sw * scale;
        dh = sh * scale;
      }
      context.clearRect(0, 0, canvas.width, canvas.height);
      try { context.drawImage(video, sx, sy, sw, sh, (canvas.width - dw) / 2, (canvas.height - dh) / 2, dw, dh); }
      catch { return; }
      canvas.dataset.frameSeconds = String(session.frame.seconds);
    };
    const resize = (): void => {
      const box = canvas.getBoundingClientRect();
      visible = box.width > 0 && box.height > 0;
      if (!visible) return;
      const width = Math.max(1, Math.min(MAX_CANVAS_WIDTH, Math.round(box.width * Math.min(window.devicePixelRatio || 1, 2))));
      const height = Math.max(1, Math.round(width * box.height / box.width));
      if (canvas.width !== width) canvas.width = width;
      if (canvas.height !== height) canvas.height = height;
      draw();
    };
    drawRef.current = draw;
    const unsubscribe = session.subscribeFrames(draw);
    const observer = new ResizeObserver(resize);
    observer.observe(canvas);
    resize();
    return () => { drawRef.current = null; observer.disconnect(); unsubscribe(); };
  }, [frame]);

  useEffect(() => { drawRef.current?.(); }, [mode, outputHeight, outputWidth, rect]);
  return <canvas ref={canvasRef} className={className} aria-hidden="true" data-stream-frame-canvas={mode} />;
}
