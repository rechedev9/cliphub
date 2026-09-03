'use client';

import { useState, type ReactNode } from 'react';
import type { StreamerBannerPlatform } from '@/lib/api/streams';
import { STREAMER_BANNER_MAX_POSITION, STREAMER_BANNER_MIN_POSITION } from '@/lib/stream-preview';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { StreamStepCard, StreamSwitch } from '@/components/streams/step-card';

/** Magenta accent for every range input in the stream editor. */
export const STREAM_SLIDER_CLASS = 'min-h-10 w-full accent-stream disabled:opacity-50';

/**
 * Streamer banner card. The banner exists when a nick is present, so the
 * switch clears the nick to turn it off and only reveals the fields to turn it
 * on; nothing is written to the plan until the user types a nick.
 */
export function StreamBannerControls({
  nick,
  nickValid,
  platform,
  position,
  hasExplicitPosition,
  slideEnabled,
  busy,
  onNickChange,
  onPlatformChange,
  onPositionChange,
  onResetPosition,
  onSlideChange,
}: {
  nick: string;
  nickValid: boolean;
  platform: StreamerBannerPlatform;
  position: number;
  hasExplicitPosition: boolean;
  slideEnabled: boolean;
  busy: boolean;
  onNickChange: (nick: string) => void;
  onPlatformChange: (platform: StreamerBannerPlatform) => void;
  onPositionChange: (position: number) => void;
  onResetPosition: () => void;
  onSlideChange: (slideEnabled: boolean) => void;
}): ReactNode {
  const [armed, setArmed] = useState(false);
  const enabled = nick.trim() !== '' || armed;

  return (
    <StreamStepCard
      title="Banner del streamer"
      control={
        <StreamSwitch
          label="Banner del streamer"
          checked={enabled}
          disabled={busy}
          onChange={(next) => {
            setArmed(next);
            if (!next && nick !== '') onNickChange('');
          }}
        />
      }
    >
      {enabled ? (
        <>
          <div className="flex gap-1.5" role="group" aria-label="Plataforma">
            {(['twitch', 'kick'] as const).map((entry) => (
              <Button
                key={entry}
                type="button"
                variant="outline"
                size="sm"
                disabled={busy}
                aria-pressed={platform === entry}
                onClick={() => onPlatformChange(entry)}
                className={`font-mono uppercase tracking-wider ${platform === entry ? 'border-stream/45 text-stream-text' : ''}`}
              >
                {entry === 'twitch' ? 'Twitch' : 'Kick'}
              </Button>
            ))}
          </div>
          <Input
            id="streamer-nick"
            aria-label="Nick del streamer"
            value={nick}
            disabled={busy}
            maxLength={25}
            pattern="[A-Za-z0-9_]{1,25}"
            aria-invalid={!nickValid}
            onChange={(e) => onNickChange(e.target.value)}
            placeholder="zacketizorcs2"
            className="h-9 font-mono"
          />
          {nickValid ? null : (
            <p role="alert" className="text-body-sm text-destructive">
              Usa solo letras, números o guiones bajos (máximo 25).
            </p>
          )}
          <div className="flex items-center justify-between gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
            <Label htmlFor="streamer-banner-position" className="font-mono text-meta uppercase tracking-wider text-fg-3">
              Posición · {Math.round(position * 100)}%
            </Label>
            <button
              type="button"
              disabled={busy}
              aria-pressed={slideEnabled}
              onClick={() => onSlideChange(!slideEnabled)}
              className={`min-h-10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring ${slideEnabled ? 'text-stream-text' : 'text-fg-3 hover:text-fg-2'}`}
            >
              Deslizamiento: {slideEnabled ? 'on' : 'off'}
            </button>
          </div>
          <input
            id="streamer-banner-position"
            type="range"
            min={STREAMER_BANNER_MIN_POSITION}
            max={STREAMER_BANNER_MAX_POSITION}
            step="0.001"
            value={position}
            disabled={busy}
            aria-valuetext={`${Math.round(position * 100)}% desde arriba`}
            onChange={(event) => onPositionChange(Number(event.target.value))}
            className={STREAM_SLIDER_CLASS}
          />
          {hasExplicitPosition ? (
            <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={onResetPosition} className="self-start">
              Restablecer posición
            </Button>
          ) : null}
        </>
      ) : (
        <p className="text-body-sm text-fg-3">Sin banner. Actívalo para añadir tu nick sobre la unión de bandas.</p>
      )}
    </StreamStepCard>
  );
}
