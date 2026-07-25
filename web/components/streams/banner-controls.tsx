'use client';

import type { ReactNode } from 'react';
import { STREAMER_BANNER_MAX_POSITION, STREAMER_BANNER_MIN_POSITION } from '@/lib/stream-preview';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

/** Magenta accent for every range input in the stream editor. */
export const STREAM_SLIDER_CLASS = 'min-h-10 w-full accent-stream disabled:opacity-50';

/**
 * The optional streamer banner: nick, vertical placement and the slide-in.
 * Position is expressed twice on purpose — a slider for coarse placement and a
 * mono percentage for the exact value that goes into the plan.
 */
export function StreamBannerControls({
  nick,
  nickValid,
  position,
  hasExplicitPosition,
  slideEnabled,
  busy,
  onNickChange,
  onPositionChange,
  onResetPosition,
  onSlideChange,
}: {
  nick: string;
  nickValid: boolean;
  position: number;
  hasExplicitPosition: boolean;
  slideEnabled: boolean;
  busy: boolean;
  onNickChange: (nick: string) => void;
  onPositionChange: (position: number) => void;
  onResetPosition: () => void;
  onSlideChange: (slideEnabled: boolean) => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-3 border-t border-border pt-5">
      <div className="flex flex-col gap-1">
        <Label htmlFor="streamer-nick" className="text-label text-fg-2">
          Banner del streamer (opcional)
        </Label>
        <p className="text-body-sm text-fg-3">
          Añade una franja morada con el nick sobre la unión entre facecam y gameplay.
        </p>
      </div>

      <Input
        id="streamer-nick"
        value={nick}
        disabled={busy}
        maxLength={25}
        pattern="[A-Za-z0-9_]{1,25}"
        aria-invalid={!nickValid}
        onChange={(e) => onNickChange(e.target.value)}
        placeholder="zacketizorcs2"
        className="max-w-sm"
      />
      {nickValid ? null : (
        <p role="alert" className="text-body-sm text-destructive">
          Usa solo letras, números o guiones bajos (máximo 25).
        </p>
      )}

      <div className="mt-1 flex max-w-xl flex-col gap-3 border-l-2 border-stream/45 bg-surface-1 py-3 pr-3 pl-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Label htmlFor="streamer-banner-position" className="text-label text-fg-2">
            Posición vertical del banner
          </Label>
          <output
            htmlFor="streamer-banner-position"
            className="font-mono text-label tabular-nums text-stream-text"
          >
            {Math.round(position * 100)}%
          </output>
        </div>
        <input
          id="streamer-banner-position"
          type="range"
          min={STREAMER_BANNER_MIN_POSITION}
          max={STREAMER_BANNER_MAX_POSITION}
          step="0.001"
          value={position}
          disabled={busy}
          aria-label="Posición vertical del banner"
          aria-valuetext={`${Math.round(position * 100)}% desde arriba`}
          onChange={(event) => onPositionChange(Number(event.target.value))}
          className={STREAM_SLIDER_CLASS}
        />
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant={slideEnabled ? 'default' : 'outline'}
            size="sm"
            disabled={busy}
            aria-pressed={slideEnabled}
            onClick={() => onSlideChange(!slideEnabled)}
          >
            {slideEnabled ? 'Deslizamiento: activado' : 'Deslizamiento: desactivado'}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={busy || !hasExplicitPosition}
            onClick={onResetPosition}
          >
            Restablecer posición
          </Button>
        </div>
        {slideEnabled ? (
          <p className="text-body-sm text-fg-3">
            La vista previa repite una entrada desde la izquierda, una pausa y la salida hacia la izquierda.
          </p>
        ) : null}
      </div>
    </div>
  );
}
