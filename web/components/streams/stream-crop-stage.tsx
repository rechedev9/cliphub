'use client';

import type { ComponentProps, ReactNode } from 'react';
import type { NormalizedRect } from '@/lib/api/streams';
import { CropPicker } from '@/components/streams/crop-picker';
import { StreamPreview } from '@/components/streams/stream-preview';

/**
 * Step 01 monitor: the facecam crop is edited on the source frame at full
 * width, with the 9:16 result beside it, instead of on a thumbnail in the rail.
 */
export function StreamCropStage({
  rect,
  disabled,
  preview,
  onChange,
}: {
  rect: NormalizedRect;
  disabled: boolean;
  preview: ComponentProps<typeof StreamPreview>;
  onChange: (rect: NormalizedRect) => void;
}): ReactNode {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center gap-6">
      <div className="flex w-full max-w-[760px] min-w-0 flex-col gap-2">
        <span className="font-mono text-meta uppercase tracking-widest text-stream-text">Recorte de facecam · fuente 16:9</span>
        <CropPicker rect={rect} onChange={onChange} disabled={disabled} />
        <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
          Arrastra el marco · esquina para redimensionar · flechas en teclado
        </span>
      </div>
      <div className="hidden h-full max-h-[420px] min-h-[160px] flex-none flex-col gap-2 @[64rem]/content:flex">
        <span className="font-mono text-meta uppercase tracking-widest text-fg-3">Salida 9:16</span>
        <StreamPreview {...preview} faceCrop={rect} />
      </div>
    </div>
  );
}
