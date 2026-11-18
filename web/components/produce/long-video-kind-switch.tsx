'use client';

import type { ReactNode } from 'react';
import { LONG_VIDEO_KIND_LABEL } from '@/lib/produce/copy';
import type { LongVideoKind } from '@/lib/produce/long-video-kind';
import { cn } from '@/lib/utils';
import { FORMAT_TONE } from '@/components/studio/format-tone';

type LongVideoKindSwitchProps = { kinds: readonly LongVideoKind[]; value: LongVideoKind; onChange: (kind: LongVideoKind) => void };

/** One player's long video: the whole match from their view, or a summary of their kills. */
export function LongVideoKindSwitch({ kinds, value, onChange }: LongVideoKindSwitchProps): ReactNode {
  return (
    <div role="group" aria-label="Tipo de vídeo" className="flex w-fit gap-1 rounded-lg border border-border-subtle bg-surface-2 p-1">
      {kinds.map((kind) => (
        <button key={kind} type="button" aria-pressed={value === kind} onClick={() => onChange(kind)}
          className={cn('min-h-9 rounded-md px-4 font-display text-body-sm font-semibold transition-colors duration-(--dur-fast) focus-visible:outline-2 focus-visible:outline-ring',
            value === kind ? FORMAT_TONE.full.fill : 'text-fg-2 hover:bg-surface-3')}>
          {LONG_VIDEO_KIND_LABEL[kind]}
        </button>
      ))}
    </div>
  );
}
