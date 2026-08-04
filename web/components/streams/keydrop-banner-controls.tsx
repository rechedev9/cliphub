'use client';

import type { ReactNode } from 'react';
import {
  KEYDROP_BANNER_DEFAULT_POSITION,
  KEYDROP_BANNER_MAX_POSITION,
  KEYDROP_BANNER_MIN_POSITION,
} from '@/lib/stream-preview';
import {
  DEFAULT_KEYDROP_CODE,
  DEFAULT_KEYDROP_END_SECONDS,
  DEFAULT_KEYDROP_START_SECONDS,
  KEYDROP_CODE_RE,
  KEYDROP_STYLES,
} from '@/lib/streams/plan';
import type { KeyDropBannerStyle } from '@/lib/api/streams';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { STREAM_SLIDER_CLASS } from '@/components/streams/banner-controls';

/**
 * Optional KeyDrop sponsor plate: style, code, vertical placement, on-screen
 * time window, and slide. The plate only burns in between start and end so it
 * can pop for the code callout without covering the whole clip.
 */
export function StreamKeyDropBannerControls({
  style,
  code,
  codeValid,
  position,
  hasExplicitPosition,
  slideEnabled,
  startSeconds,
  endSeconds,
  clipDurationSeconds,
  busy,
  onStyleChange,
  onCodeChange,
  onPositionChange,
  onResetPosition,
  onSlideChange,
  onStartChange,
  onEndChange,
}: {
  style: KeyDropBannerStyle | '';
  code: string;
  codeValid: boolean;
  position: number;
  hasExplicitPosition: boolean;
  slideEnabled: boolean;
  startSeconds: number;
  endSeconds: number;
  /** Longest clip length used to clamp the time window; 0 = no clamp. */
  clipDurationSeconds: number;
  busy: boolean;
  onStyleChange: (style: KeyDropBannerStyle | '') => void;
  onCodeChange: (code: string) => void;
  onPositionChange: (position: number) => void;
  onResetPosition: () => void;
  onSlideChange: (slideEnabled: boolean) => void;
  onStartChange: (startSeconds: number) => void;
  onEndChange: (endSeconds: number) => void;
}): ReactNode {
  const enabled = style !== '';
  const maxT = clipDurationSeconds > 0 ? clipDurationSeconds : Math.max(endSeconds, DEFAULT_KEYDROP_END_SECONDS, 30);
  const rangeValid = startSeconds >= 0 && endSeconds > startSeconds && (clipDurationSeconds <= 0 || endSeconds <= clipDurationSeconds + 0.001);

  return (
    <div className="flex flex-col gap-3 border-t border-border pt-5">
      <div className="flex flex-col gap-1">
        <Label className="text-label text-fg-2">Banner KeyDrop (opcional)</Label>
        <p className="text-body-sm text-fg-3">
          Placa de sponsor con código. Elige cuándo entra y sale; no tiene por qué quedarse todo el clip.
        </p>
      </div>

      <div className="flex flex-wrap gap-2" role="group" aria-label="Estilo del banner KeyDrop">
        <Button
          type="button"
          size="sm"
          variant={style === '' ? 'default' : 'outline'}
          disabled={busy}
          aria-pressed={style === ''}
          onClick={() => onStyleChange('')}
        >
          Sin KeyDrop
        </Button>
        {KEYDROP_STYLES.map((entry) => (
          <Button
            key={entry.value}
            type="button"
            size="sm"
            variant={style === entry.value ? 'default' : 'outline'}
            disabled={busy}
            aria-pressed={style === entry.value}
            onClick={() => onStyleChange(entry.value)}
          >
            {entry.label}
            <span className="ml-1.5 text-fg-3">{entry.subtitle}</span>
          </Button>
        ))}
      </div>

      {enabled ? (
        <div className="mt-1 flex max-w-xl flex-col gap-3 border-l-2 border-amber-500/50 bg-surface-1 py-3 pr-3 pl-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="keydrop-code" className="text-label text-fg-2">
              Código
            </Label>
            <Input
              id="keydrop-code"
              value={code}
              disabled={busy}
              maxLength={16}
              aria-invalid={!codeValid}
              onChange={(e) => onCodeChange(e.target.value.toUpperCase())}
              placeholder={DEFAULT_KEYDROP_CODE}
              className="max-w-sm font-mono uppercase"
            />
            {codeValid ? (
              <p className="text-body-sm text-fg-3">
                Se renderiza como <span className="font-mono text-fg-1">CODE: {(code.trim() || DEFAULT_KEYDROP_CODE)}</span>
              </p>
            ) : (
              <p role="alert" className="text-body-sm text-destructive">
                Usa 1–16 letras, números, guiones o guiones bajos (sin espacios).
              </p>
            )}
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Label className="text-label text-fg-2">Visible en el clip</Label>
              <output className="font-mono text-label tabular-nums text-amber-200">
                {startSeconds.toFixed(1)}s → {endSeconds.toFixed(1)}s
              </output>
            </div>
            <p className="text-body-sm text-fg-3">
              Tiempo desde el inicio de cada clip (por defecto {DEFAULT_KEYDROP_START_SECONDS}–{DEFAULT_KEYDROP_END_SECONDS}s).
            </p>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="keydrop-start" className="text-label text-fg-3">
                  Entra (s)
                </Label>
                <Input
                  id="keydrop-start"
                  type="number"
                  min={0}
                  max={maxT}
                  step={0.1}
                  value={Number.isFinite(startSeconds) ? startSeconds : 0}
                  disabled={busy}
                  onChange={(e) => onStartChange(Number(e.target.value))}
                  className="max-w-[10rem] font-mono"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="keydrop-end" className="text-label text-fg-3">
                  Sale (s)
                </Label>
                <Input
                  id="keydrop-end"
                  type="number"
                  min={0.1}
                  max={maxT}
                  step={0.1}
                  value={Number.isFinite(endSeconds) ? endSeconds : DEFAULT_KEYDROP_END_SECONDS}
                  disabled={busy}
                  onChange={(e) => onEndChange(Number(e.target.value))}
                  className="max-w-[10rem] font-mono"
                />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="keydrop-window-start" className="sr-only">
                Inicio del banner en el clip
              </Label>
              <input
                id="keydrop-window-start"
                type="range"
                min={0}
                max={maxT}
                step={0.1}
                value={Math.min(startSeconds, maxT)}
                disabled={busy}
                aria-label="Segundo de entrada del banner KeyDrop"
                onChange={(e) => onStartChange(Number(e.target.value))}
                className={STREAM_SLIDER_CLASS}
              />
              <input
                id="keydrop-window-end"
                type="range"
                min={0}
                max={maxT}
                step={0.1}
                value={Math.min(endSeconds, maxT)}
                disabled={busy}
                aria-label="Segundo de salida del banner KeyDrop"
                onChange={(e) => onEndChange(Number(e.target.value))}
                className={STREAM_SLIDER_CLASS}
              />
            </div>
            {rangeValid ? null : (
              <p role="alert" className="text-body-sm text-destructive">
                La salida debe ser mayor que la entrada
                {clipDurationSeconds > 0 ? ` y como máximo ${clipDurationSeconds.toFixed(1)}s` : ''}.
              </p>
            )}
          </div>

          <div className="flex flex-wrap items-center justify-between gap-2">
            <Label htmlFor="keydrop-banner-position" className="text-label text-fg-2">
              Posición vertical
            </Label>
            <output
              htmlFor="keydrop-banner-position"
              className="font-mono text-label tabular-nums text-amber-200"
            >
              {Math.round(position * 100)}%
            </output>
          </div>
          <input
            id="keydrop-banner-position"
            type="range"
            min={KEYDROP_BANNER_MIN_POSITION}
            max={KEYDROP_BANNER_MAX_POSITION}
            step="0.001"
            value={position}
            disabled={busy}
            aria-label="Posición vertical del banner KeyDrop"
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
              Restablecer posición ({Math.round(KEYDROP_BANNER_DEFAULT_POSITION * 100)}%)
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function isKeyDropCodeValid(code: string): boolean {
  const trimmed = code.trim();
  return trimmed === '' || KEYDROP_CODE_RE.test(trimmed);
}
