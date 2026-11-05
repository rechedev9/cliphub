'use client';

import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { STREAM_SLIDER_CLASS } from '@/components/streams/banner-controls';
import { StreamStepCard, StreamSwitch } from '@/components/streams/step-card';
import { STREAMER_BANNER_MAX_POSITION, STREAMER_BANNER_MIN_POSITION } from '@/lib/stream-preview';

/** Redraws the streamer's own kill feed notices centered on the Short. */
export function StreamKillFeedControls({
  centered,
  position,
  hasExplicitPosition,
  busy,
  onCenteredChange,
  onPositionChange,
  onResetPosition,
}: {
  centered: boolean;
  /** Null when the source size is unknown and the render picks the spot. */
  position: number | null;
  hasExplicitPosition: boolean;
  busy: boolean;
  onCenteredChange: (centered: boolean) => void;
  onPositionChange: (position: number) => void;
  onResetPosition: () => void;
}): ReactNode {
  return (
    <StreamStepCard
      title="Kill feed centrada"
      control={<StreamSwitch label="Kill feed centrada" checked={centered} disabled={busy} onChange={onCenteredChange} />}
    >
      {centered ? (
        <>
          <p className="text-body-sm text-fg-3">
            Tus kills aparecen centradas bajo el marcador, una debajo de otra, como en la partida.
          </p>
          {position === null ? null : (
            <>
              <Label htmlFor="kill-feed-position" className="text-meta text-fg-3">
                Altura · {Math.round(position * 100)}%{hasExplicitPosition ? '' : ' (bajo el marcador)'}
              </Label>
              <input
                id="kill-feed-position"
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
            </>
          )}
          {hasExplicitPosition ? (
            <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={onResetPosition} className="self-start">
              Volver bajo el marcador
            </Button>
          ) : null}
        </>
      ) : (
        <p className="text-body-sm text-fg-3">Desactivada. El recorte vertical deja fuera la kill feed original.</p>
      )}
    </StreamStepCard>
  );
}
