'use client';

import type { ReactNode } from 'react';
import {
  KEYDROP_BANNER_DEFAULT_POSITION,
  KEYDROP_BANNER_MAX_POSITION,
  KEYDROP_BANNER_MIN_POSITION,
} from '@/lib/stream-preview';
import {
  AFFILIATE_FAMILY_CATALOG,
  DEFAULT_KEYDROP_CODE,
  KEYDROP_CODE_RE,
  affiliateDisplayLabel,
  isKeyDropStyle,
  stylesForFamily,
  type AffiliateFamily,
} from '@/lib/api/types';
import { DEFAULT_KEYDROP_END_SECONDS, DEFAULT_KEYDROP_START_SECONDS } from '@/lib/streams/plan';
import type { KeyDropBannerStyle } from '@/lib/api/streams';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { STREAM_SLIDER_CLASS } from '@/components/streams/banner-controls';
import { StreamStepCard, StreamSwitch } from '@/components/streams/step-card';

export function StreamKeyDropBannerControls({
  family,
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
  onFamilyChange,
  onStyleChange,
  onCodeChange,
  onPositionChange,
  onResetPosition,
  onSlideChange,
  onStartChange,
  onEndChange,
}: {
  family: AffiliateFamily | '';
  style: KeyDropBannerStyle | '';
  code: string;
  codeValid: boolean;
  position: number;
  hasExplicitPosition: boolean;
  slideEnabled: boolean;
  startSeconds: number;
  endSeconds: number;
  clipDurationSeconds: number;
  busy: boolean;
  onFamilyChange: (family: AffiliateFamily) => void;
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

  const familyLabel = AFFILIATE_FAMILY_CATALOG.find((entry) => entry.id === family)?.label ?? 'KeyDrop';
  const firstStyle = stylesForFamily(family || 'KEYDROP')[0]?.id ?? '';

  return (
    <StreamStepCard
      title={`${familyLabel} · afiliado`}
      control={
        <StreamSwitch
          label="Banner afiliado"
          checked={enabled}
          disabled={busy}
          onChange={(next) => {
            if (!next) onStyleChange('');
            else if (isKeyDropStyle(firstStyle)) onStyleChange(firstStyle);
          }}
        />
      }
    >
      {enabled ? (
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap gap-1.5" role="group" aria-label="Familia del banner">
            {AFFILIATE_FAMILY_CATALOG.map((entry) => (
              <Button
                key={entry.id}
                type="button"
                size="sm"
                variant="outline"
                disabled={busy}
                aria-pressed={family === entry.id}
                onClick={() => onFamilyChange(entry.id)}
                className={`font-mono uppercase tracking-wider ${family === entry.id ? 'border-stream/45 text-stream-text' : ''}`}
              >
                {entry.label}
              </Button>
            ))}
          </div>

          <div className="flex flex-wrap gap-1.5" role="group" aria-label="Estilo del banner">
            {stylesForFamily(family || 'KEYDROP').map((entry) => (
              <Button
                key={entry.id}
                type="button"
                size="sm"
                variant="outline"
                disabled={busy}
                aria-pressed={style === entry.id}
                onClick={() => {
                  if (isKeyDropStyle(entry.id)) onStyleChange(entry.id);
                }}
                className={`font-mono uppercase tracking-wider ${style === entry.id ? 'border-stream/45 text-stream-text' : ''}`}
              >
                {entry.label}
                <span className="text-fg-3">{entry.subtitle}</span>
              </Button>
            ))}
          </div>

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
              className="h-9 font-mono uppercase"
            />
            {codeValid ? (
              <p className="text-body-sm text-fg-3">
                Se renderiza como <span className="font-mono text-fg-1">{affiliateDisplayLabel(family, style, code)}</span>
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
              <output className="font-mono text-label tabular-nums text-stream-text">
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
                  className="h-9 font-mono"
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
                  className="h-9 font-mono"
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
                aria-label="Segundo de entrada del banner"
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
                aria-label="Segundo de salida del banner"
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
              className="font-mono text-label tabular-nums text-stream-text"
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
            aria-label="Posición vertical del banner"
            aria-valuetext={`${Math.round(position * 100)}% desde arriba`}
            onChange={(event) => onPositionChange(Number(event.target.value))}
            className={STREAM_SLIDER_CLASS}
          />
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              aria-pressed={slideEnabled}
              onClick={() => onSlideChange(!slideEnabled)}
              className={`font-mono uppercase tracking-wider ${slideEnabled ? 'border-stream/45 text-stream-text' : ''}`}
            >
              Deslizamiento: {slideEnabled ? 'on' : 'off'}
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
      ) : (
        <p className="text-body-sm text-fg-3">Sin placa de sponsor. Actívala para elegir estilo, código y ventana.</p>
      )}
    </StreamStepCard>
  );
}

export function isKeyDropCodeValid(code: string): boolean {
  const trimmed = code.trim();
  return trimmed === '' || KEYDROP_CODE_RE.test(trimmed);
}
