'use client';

import type { ReactNode } from 'react';
import {
  KEYDROP_BANNER_DEFAULT_POSITION,
  KEYDROP_BANNER_MAX_POSITION,
  KEYDROP_BANNER_MIN_POSITION,
} from '@/lib/stream-preview';
import {
  DEFAULT_KEYDROP_CODE,
  KEYDROP_CODE_RE,
  KEYDROP_STYLES,
} from '@/lib/streams/plan';
import type { KeyDropBannerStyle } from '@/lib/api/streams';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { STREAM_SLIDER_CLASS } from '@/components/streams/banner-controls';

/**
 * Optional KeyDrop sponsor plate: style picker, editable code, vertical
 * placement and slide-in. Mirrors StreamBannerControls so both banners feel
 * like the same product surface.
 */
export function StreamKeyDropBannerControls({
  style,
  code,
  codeValid,
  position,
  hasExplicitPosition,
  slideEnabled,
  busy,
  onStyleChange,
  onCodeChange,
  onPositionChange,
  onResetPosition,
  onSlideChange,
}: {
  style: KeyDropBannerStyle | '';
  code: string;
  codeValid: boolean;
  position: number;
  hasExplicitPosition: boolean;
  slideEnabled: boolean;
  busy: boolean;
  onStyleChange: (style: KeyDropBannerStyle | '') => void;
  onCodeChange: (code: string) => void;
  onPositionChange: (position: number) => void;
  onResetPosition: () => void;
  onSlideChange: (slideEnabled: boolean) => void;
}): ReactNode {
  const enabled = style !== '';
  return (
    <div className="flex flex-col gap-3 border-t border-border pt-5">
      <div className="flex flex-col gap-1">
        <Label className="text-label text-fg-2">Banner KeyDrop (opcional)</Label>
        <p className="text-body-sm text-fg-3">
          Superpone la placa de sponsor con tu código. Puedes combinarla con el banner de Twitch.
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
