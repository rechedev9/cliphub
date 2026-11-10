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
 * The export row under the map: the tramo (its own track for one round, the timeline's when superposed),
 * how long it lasts, and the GIF recording with its progress.
 */
export function GifPanel({
  gif,
  tramo,
  multi,
  speed,
  onClose,
}: {
  gif: ReplayGif;
  tramo: TramoControl;
  multi: boolean;
  speed: number;
  onClose: () => void;
}): ReactNode {
  const busy = gif.busy;
  const line = stateLine(gif.state);
  const failed = gif.state.kind === 'failed';
  let summary = gif.blocked ?? `${Math.max(1, Math.round(gif.seconds))} s${speed === 1 ? '' : ` a ${formatRate(speed)}`}, con el encuadre, las capas y el dibujo que ves.`;
  if (multi && gif.blocked === null) summary = `${tramoLabel(tramo.tramo)} · ${summary}`;
  const percent = gif.state.kind === 'recording' ? gif.state.percent : null;
  return (
    <section
      aria-label="Exportar el tramo"
      className="flex shrink-0 flex-col gap-1.5 border-t border-border-subtle bg-surface-1 px-2 py-1.5"
    >
      {multi ? null : <TramoBar tramo={tramo.tramo} onChange={tramo.onChange} />}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <p className="min-w-[10rem] flex-1 text-meta text-fg-2">{summary}</p>
        <div className="flex items-center gap-1.5">
          {busy ? (
            <Button type="button" size="xs" variant="ghost" onClick={gif.cancel} disabled={gif.state.kind !== 'recording'}>
              Cancelar
            </Button>
          ) : (
            <Button type="button" size="xs" variant="outline" onClick={gif.start} disabled={gif.blocked !== null} title="Grabar el GIF del tramo (G)">
              <Film aria-hidden />
              Grabar GIF
            </Button>
          )}
          <Button type="button" size="icon-xs" variant="ghost" onClick={onClose} disabled={busy} aria-label="Cerrar la exportación del tramo">
            <X aria-hidden />
          </Button>
        </div>
      </div>
      {percent === null ? null : <Progress size="xs" value={percent} aria-label="Progreso de la exportación" />}
      <p role="status" className={failed ? 'text-meta text-destructive empty:hidden' : 'text-meta text-fg-2 empty:hidden'}>
        {line}
      </p>
    </section>
  );
}
