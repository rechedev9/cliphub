'use client';

import type { ReactNode } from 'react';
import { Film, X } from 'lucide-react';
import { formatRate } from '@/lib/format';
import { tramoLabel } from '@/lib/replay/tramo';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { TramoBar } from './tramo-bar';
import type { TramoControl } from './tramo-bar';
import type { GifState, ReplayGif } from './use-replay-gif';

function stateLine(state: GifState): string | null {
  if (state.kind === 'picking') return 'Elige dónde guardarlo…';
  if (state.kind === 'recording') return `Creando el GIF… ${state.percent} %`;
  if (state.kind === 'saving') return 'Guardando el GIF…';
  if (state.kind === 'done' || state.kind === 'failed') return state.message;
  return null;
}

/**
 * The GIF row under the map bar: which tramo goes in (on its own track while
 * one round plays; superposed rounds take the timeline's), how long it lasts,
 * and the export with its progress.
 */
export function GifPanel({ gif, tramo, multi, speed, onClose }: { gif: ReplayGif; tramo: TramoControl; multi: boolean; speed: number; onClose: () => void }): ReactNode {
  const line = stateLine(gif.state);
  let summary = gif.blocked ?? `GIF de ${Math.max(1, Math.round(gif.seconds))} s${speed === 1 ? '' : ` a ${formatRate(speed)}`}, con el encuadre, las capas y el dibujo que ves.`;
  if (multi && gif.blocked === null) summary = `${tramoLabel(tramo.tramo)} · ${summary}`;
  return (
    <section aria-label="GIF del tramo" className="flex shrink-0 flex-col gap-2 border-b border-border-subtle bg-surface-1 px-2 py-2">
      {multi ? null : <TramoBar tramo={tramo.tramo} onChange={tramo.onChange} />}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <p className="min-w-0 flex-1 text-meta text-fg-2">{summary}</p>
        <div className="flex items-center gap-1.5">
          {gif.busy ? (
            <Button type="button" size="xs" variant="ghost" onClick={gif.cancel} disabled={gif.state.kind !== 'recording'}>
              Cancelar
            </Button>
          ) : (
            <Button type="button" size="xs" onClick={gif.start} disabled={gif.blocked !== null} title="Exportar el GIF del tramo (G)">
              <Film aria-hidden />
              Exportar GIF
            </Button>
          )}
          <Button type="button" size="icon-xs" variant="ghost" onClick={onClose} disabled={gif.busy} aria-label="Cerrar el GIF del tramo">
            <X aria-hidden />
          </Button>
        </div>
      </div>
      {gif.state.kind === 'recording' ? <Progress size="xs" value={gif.state.percent} aria-label="Progreso del GIF" /> : null}
      <p role="status" className={gif.state.kind === 'failed' ? 'text-meta text-destructive empty:hidden' : 'text-meta text-fg-2 empty:hidden'}>
        {line}
      </p>
    </section>
  );
}
