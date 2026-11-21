'use client';

import type { ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import { Kbd } from '@/components/streams/kbd';
import { MomentEdgesEditor } from '@/components/streams/moment-edges-editor';
import type { StreamClipRange, StreamPlay } from '@/lib/api/streams';
import { suggestedCut, type MomentEdge } from '@/lib/streams/moment-edges';
import { MOMENTS_KEY } from '@/lib/streams/moments-keys';
import { clipOutputDuration, formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { cn } from '@/lib/utils';

export function StreamClipEditor({
  clips,
  plays,
  sourceDuration,
  selectedClipId,
  onChange,
  onSelect,
  onRemove,
  onNudgeEdge,
  onSuggest,
  disabled,
  onPlay,
}: {
  clips: StreamClipRange[];
  plays: readonly StreamPlay[];
  sourceDuration: number;
  selectedClipId: string | null;
  onChange: (clips: StreamClipRange[]) => void;
  onSelect: (clip: StreamClipRange) => void;
  onRemove: (clip: StreamClipRange) => void;
  onNudgeEdge: (change: { clipId: string; edge: MomentEdge; delta: number }) => void;
  onSuggest: (clip: StreamClipRange) => void;
  disabled: boolean;
  onPlay: () => void;
}): ReactNode {
  const updateClip = (id: string, patch: Partial<StreamClipRange>) =>
    onChange(clips.map((c) => (c.id === id ? { ...c, ...patch } : c)));

  if (clips.length === 0) {
    return (
      <p className="border border-dashed border-border-subtle p-3.5 text-center text-body-sm text-fg-2">
        {sourceDuration > 0
          ? 'Muévete por la línea de tiempo y pulsa «Marcar momento aquí» para quedarte con esa parte.'
          : 'Espera a que cargue el vídeo original antes de añadir momentos.'}
      </p>
    );
  }

  const index = Math.max(
    0,
    clips.findIndex((clip) => clip.id === selectedClipId),
  );
  const clip = clips[index];
  return (
    <div className="flex flex-col gap-3">
      <ul aria-label="Momento seleccionado">
        <ClipCard
          key={clip.id}
          clip={clip}
          index={index}
          canSuggest={suggestedCut({ clips, clipId: clip.id, plays, sourceDuration }) !== null}
          selected={clip.id === selectedClipId}
          disabled={disabled}
          onSelect={() => onSelect(clip)}
          onRemove={() => onRemove(clip)}
          onClipChange={(patch) => updateClip(clip.id, patch)}
          onNudge={(change) => onNudgeEdge({ clipId: clip.id, ...change })}
          onSuggest={() => onSuggest(clip)}
          onPlay={onPlay}
        />
      </ul>
    </div>
  );
}

function ClipCard({
  clip,
  index,
  canSuggest,
  selected,
  disabled,
  onSelect,
  onRemove,
  onClipChange,
  onNudge,
  onSuggest,
  onPlay,
}: {
  clip: StreamClipRange;
  index: number;
  canSuggest: boolean;
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
  onRemove: () => void;
  onClipChange: (patch: Partial<StreamClipRange>) => void;
  onNudge: (change: { edge: MomentEdge; delta: number }) => void;
  onSuggest: () => void;
  onPlay: () => void;
}): ReactNode {
  const number = String(index + 1).padStart(2, '0');
  return (
    <li
      className={cn(
        'flex flex-col gap-2.5 border p-3.5 transition-colors duration-(--dur-fast) ease-standard',
        selected ? 'border-stream/45 bg-surface-3' : 'border-border bg-surface-2',
      )}
    >
      <div className="flex items-center gap-2.5">
        <button
          type="button"
          onClick={onSelect}
          aria-pressed={selected}
          className="flex min-h-10 min-w-0 flex-1 items-center gap-2.5 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          <span
            aria-hidden
            className={cn(
              'grid size-7 shrink-0 place-items-center border text-label tabular-nums',
              selected ? cn('border-format-stream text-format-stream-foreground', FORMAT_TONE.stream.bg) : 'border-format-stream/45 text-format-stream',
            )}
          >
            {number}
          </span>
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="truncate font-display text-label font-semibold text-fg-1">
              {clip.title?.trim() || `Momento ${index + 1}`}
            </span>
            <span className="truncate text-meta text-fg-3">
              {formatStreamClock(clip.start_seconds)} → {formatStreamClock(clip.end_seconds)}
              {formatStreamClock(clipOutputDuration(clip)) === formatStreamClock(clip.end_seconds - clip.start_seconds)
                ? null
                : ` · ${formatStreamClock(clipOutputDuration(clip))} con el cambio de velocidad`}
            </span>
          </span>
        </button>
        <Button type="button" variant="outline" size="sm" disabled={disabled} onClick={onRemove} aria-label={`Quitar momento ${number}`} aria-keyshortcuts="Delete">
          <Trash2 aria-hidden />
          Quitar
          <Kbd>{MOMENTS_KEY.remove}</Kbd>
        </Button>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor={`${clip.id}-title`} className="text-label text-fg-2">
          Nombre del momento
        </Label>
        <Input
          id={`${clip.id}-title`}
          value={clip.title ?? ''}
          disabled={disabled}
          onChange={(e) => onClipChange({ title: e.target.value })}
          placeholder={`Momento ${index + 1}`}
          className="h-9"
        />
      </div>

      <MomentEdgesEditor start={clip.start_seconds} end={clip.end_seconds} disabled={disabled} onNudge={onNudge} />
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" disabled={disabled} onClick={onPlay}>
          Ver este momento
        </Button>
        {canSuggest ? (
          <Button variant="outline" size="sm" disabled={disabled} onClick={onSuggest}>
            Volver a los bordes de la jugada
          </Button>
        ) : null}
      </div>
    </li>
  );
}
