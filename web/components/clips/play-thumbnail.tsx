import type { ReactNode } from 'react';
import { Crosshair } from 'lucide-react';
import type { Play } from '@/lib/api/types';
import { CoverImage } from '@/components/studio/cover-image';
import { cn } from '@/lib/utils';

/** Use a captured frame when available; otherwise show real event data, not cover art. */
export function PlayThumbnail({ play, compact = false, className }: { play: Play; compact?: boolean; className?: string }): ReactNode {
  return <span aria-hidden className={cn('relative flex aspect-video shrink-0 items-center justify-center overflow-hidden border border-border-strong bg-surface-3', className)}>
    <span className="flex flex-col items-center gap-1 text-fg-2">
      <span className="flex items-center gap-1.5 font-mono text-body font-semibold"><Crosshair className="size-4" />{play.kills}</span>
      {!compact ? <span className="text-meta tracking-normal">Sin captura</span> : null}
    </span>
    <CoverImage src={play.thumbnailUrl} className="absolute inset-0" />
  </span>;
}
