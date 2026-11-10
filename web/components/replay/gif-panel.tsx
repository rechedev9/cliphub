'use client';

import { useId, type ReactNode } from 'react';
import { Clapperboard, Film, X } from 'lucide-react';
import { formatRate } from '@/lib/format';
import { tramoLabel } from '@/lib/replay/tramo';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Switch } from '@/components/ui/switch';
import { TramoBar } from './tramo-bar';
import type { TramoControl } from './tramo-bar';
import type { GifState, ReplayGif } from './use-replay-gif';
import type { ReplayVideo, VideoState } from './use-replay-video';

function stateLine(state: GifState): string | null {
  if (state.kind === 'picking') return 'Elige dónde guardarlo…';
  if (state.kind === 'recording') return `Creando el GIF… ${state.percent} %`;
  if (state.kind === 'saving') return 'Guardando el GIF…';
  if (state.kind === 'done' || state.kind === 'failed') return state.message;
  return null;
}

function videoLine(state: VideoState): string | null {
  if (state.kind === 'picking') return 'Elige dónde guardarlo…';
  if (state.kind === 'rendering') return `Dibujando el MP4… ${state.percent} %`;
  if (state.kind === 'encoding') return 'Codificando el MP4…';
  if (state.kind === 'saving') return 'Guardando el MP4…';
  if (state.kind === 'done' || state.kind === 'failed') return state.message;
  return null;
}

/** The line under the buttons: the export running now, else the last one that ended. */
function statusOf(gif: ReplayGif, video: ReplayVideo): { line: string | null; failed: boolean } {
  if (video.state.kind !== 'idle' && (video.busy || !gif.busy)) return { line: videoLine(video.state), failed: video.state.kind === 'failed' };
  return { line: stateLine(gif.state), failed: gif.state.kind === 'failed' };
}

/**
 * The export row under the map: the tramo (its own track for one round, the timeline's when superposed),
 * how long it lasts, and the GIF and MP4 exports with their progress; the MP4 can carry the team voice.
 */
export function GifPanel({
  gif,
  video,
  tramo,
  multi,
  speed,
  onClose,
}: {
  gif: ReplayGif;
  video: ReplayVideo;
  tramo: TramoControl;
  multi: boolean;
  speed: number;
  onClose: () => void;
}): ReactNode {
  const voiceId = useId();
  const busy = gif.busy || video.busy;
  const status = statusOf(gif, video);
  let summary = gif.blocked ?? `${Math.max(1, Math.round(gif.seconds))} s${speed === 1 ? '' : ` a ${formatRate(speed)}`}, con el encuadre, las capas y el dibujo que ves.`;
  if (multi && gif.blocked === null) summary = `${tramoLabel(tramo.tramo)} · ${summary}`;
  let percent: number | null = null;
  if (gif.state.kind === 'recording') percent = gif.state.percent;
  if (video.state.kind === 'rendering') percent = video.state.percent;
  return (
    <section
      aria-label="Exportar el tramo"
      className="flex min-h-0 flex-col gap-1.5 overflow-y-auto overscroll-contain border-t border-border-subtle bg-surface-1 px-2 py-1.5 [scrollbar-width:thin]"
    >
      {multi ? null : <TramoBar compact tramo={tramo.tramo} onChange={tramo.onChange} />}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <p className="min-w-[10rem] flex-1 text-meta text-fg-2">{summary}</p>
        <div className="flex items-center gap-2" title={video.voice.hint ?? undefined}>
          <Switch
            aria-labelledby={voiceId}
            checked={video.includeVoice}
            onCheckedChange={video.setIncludeVoice}
            disabled={!video.voice.available || busy}
          />
          <span id={voiceId} className={video.voice.available ? 'text-meta text-fg-2' : 'text-meta text-fg-4'}>
            Incluir voz del equipo
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          {busy ? (
            <Button
              type="button"
              size="xs"
              variant="ghost"
              onClick={gif.busy ? gif.cancel : video.cancel}
              disabled={gif.busy ? gif.state.kind !== 'recording' : video.state.kind !== 'rendering' && video.state.kind !== 'encoding'}
            >
              Cancelar
            </Button>
          ) : (
            <>
              <Button type="button" size="xs" variant="outline" onClick={gif.start} disabled={gif.blocked !== null} title="Exportar el GIF del tramo (G)">
                <Film aria-hidden />
                Exportar GIF
              </Button>
              <Button type="button" size="xs" onClick={video.start} disabled={video.blocked !== null} title={video.blocked ?? 'Exportar el tramo como vídeo MP4'}>
                <Clapperboard aria-hidden />
                Exportar MP4
              </Button>
            </>
          )}
          <Button type="button" size="icon-xs" variant="ghost" onClick={onClose} disabled={busy} aria-label="Cerrar la exportación del tramo">
            <X aria-hidden />
          </Button>
        </div>
      </div>
      {percent === null ? null : <Progress size="xs" value={percent} aria-label="Progreso de la exportación" />}
      <p role="status" className={status.failed ? 'text-meta text-destructive empty:hidden' : 'text-meta text-fg-2 empty:hidden'}>
        {status.line}
      </p>
    </section>
  );
}
