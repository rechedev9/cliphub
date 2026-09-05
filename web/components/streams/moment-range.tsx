'use client';
import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { NumberField } from '@/components/streams/number-field';
import { formatStreamClock } from '@/lib/streams/plan';
export function MomentRange({
  id,
  start,
  end,
  duration,
  playhead,
  disabled,
  invalid,
  onChange,
}: {
  id: string;
  start: number;
  end: number;
  duration: number;
  playhead: number;
  disabled: boolean;
  invalid?: boolean;
  onChange: (patch: { start_seconds?: number; end_seconds?: number }) => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-2">
      <div className="grid grid-cols-2 gap-3">
        <NumberField
          id={`${id}-start`}
          label="Inicio (s)"
          value={start}
          max={duration}
          disabled={disabled}
          invalid={invalid}
          onChange={(value) => onChange({ start_seconds: value })}
        />
        <NumberField
          id={`${id}-end`}
          label="Fin (s)"
          value={end}
          max={duration}
          disabled={disabled}
          invalid={invalid}
          onChange={(value) => onChange({ end_seconds: value })}
        />
        <Button variant="outline" size="sm" disabled={disabled} onClick={() => onChange({ start_seconds: playhead })}>
          Marcar inicio aquí
        </Button>
        <Button variant="outline" size="sm" disabled={disabled} onClick={() => onChange({ end_seconds: playhead })}>
          Marcar final aquí
        </Button>
      </div>
      <label className="flex items-center gap-2 text-label text-fg-3">
        Inicio
        <input
          aria-label="Arrastrar inicio del momento"
          type="range"
          min={0}
          max={duration || 1}
          step={0.1}
          value={start}
          disabled={disabled}
          className="min-w-0 flex-1 accent-stream"
          onChange={(e) => onChange({ start_seconds: Math.min(Number(e.target.value), Math.max(0, end - 0.1)) })}
        />
      </label>
      <label className="flex items-center gap-2 text-label text-fg-3">
        Final
        <input
          aria-label="Arrastrar final del momento"
          type="range"
          min={0}
          max={duration || 1}
          step={0.1}
          value={end}
          disabled={disabled}
          className="min-w-0 flex-1 accent-stream"
          onChange={(e) => onChange({ end_seconds: Math.max(Number(e.target.value), Math.min(duration, start + 0.1)) })}
        />
      </label>
      <p className="text-label text-fg-2">Duración del momento: {formatStreamClock(Math.max(0, end - start))}</p>
    </div>
  );
}
