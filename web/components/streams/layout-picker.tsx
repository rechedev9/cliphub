'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, CircleCheck } from 'lucide-react';
import { STREAM_VARIANTS, type NormalizedRect, type StreamVariant } from '@/lib/api/streams';
import { DEFAULT_FACE_CROP } from '@/lib/streams/plan';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { SelectableCard } from '@/components/studio/selectable-card';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { CropPicker } from '@/components/streams/crop-picker';
import { LayoutGlyph } from '@/components/streams/layout-glyph';
import { cn } from '@/lib/utils';

/**
 * Output shape and facecam framing. The variant cards are real `aria-pressed`
 * controls with a 3D proxy of the band stack each one produces, so the choice
 * previews its result instead of naming it.
 */
export function StreamLayoutPicker({
  variant,
  faceCrop,
  faceCropReviewed,
  needsFaceCrop,
  busy,
  onVariantChange,
  onFaceCropChange,
  onConfirmFaceCrop,
}: {
  variant: StreamVariant;
  faceCrop?: NormalizedRect;
  faceCropReviewed: boolean;
  needsFaceCrop: boolean;
  busy: boolean;
  onVariantChange: (variant: StreamVariant) => void;
  onFaceCropChange: (rect: NormalizedRect) => void;
  onConfirmFaceCrop: () => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionEyebrow label="LAYOUT" />
        <StatusTag>SALIDA 1080×1920</StatusTag>
      </div>

      <div className="grid gap-3 @[30rem]/editor:grid-cols-2 @[44rem]/editor:grid-cols-3">
        {STREAM_VARIANTS.map((v) => {
          const selected = v.value === variant;
          return (
            <SelectableCard
              key={v.value}
              selected={selected}
              onSelect={() => onVariantChange(v.value)}
              disabled={busy}
              tone="stream"
              tilt={false}
              className="flex-row items-center gap-3 p-3"
            >
              <LayoutGlyph variant={v.value} selected={selected} />
              <span className="flex min-w-0 flex-col gap-1 text-left">
                <span className="font-display text-label font-bold uppercase text-fg-1">{v.label}</span>
                <span
                  className={cn(
                    'font-mono text-meta uppercase tracking-wider',
                    selected ? 'text-stream-text' : 'text-fg-3',
                  )}
                >
                  {v.subtitle}
                </span>
              </span>
            </SelectableCard>
          );
        })}
      </div>

      {needsFaceCrop ? (
        <div className="flex flex-col gap-3">
          <Label className="text-label text-fg-2">
            Recorte de facecam: arrastra para mover o usa las flechas; ajusta la esquina para redimensionar
          </Label>
          <CropPicker
            rect={faceCrop ?? DEFAULT_FACE_CROP}
            onChange={onFaceCropChange}
            disabled={busy}
          />
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              size="sm"
              variant={faceCropReviewed ? 'outline' : 'default'}
              disabled={busy}
              onClick={onConfirmFaceCrop}
            >
              <CircleCheck className="size-4" />
              {faceCropReviewed ? 'RECORTE CONFIRMADO' : 'CONFIRMAR RECORTE DE FACECAM'}
            </Button>
            {faceCropReviewed ? null : (
              <p
                role="alert"
                className="flex min-w-56 flex-1 items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning"
              >
                <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
                Verifica que el marco contiene una cara. El recorte inicial es solo una guía y podría coincidir con el radar.
              </p>
            )}
          </div>
        </div>
      ) : (
        <p className="text-body-sm text-fg-2">
          No hace falta recorte de facecam: este layout renderiza el gameplay a pantalla completa.
        </p>
      )}
    </div>
  );
}
