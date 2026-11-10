'use client';

import type { ReactNode } from 'react';
import { Info } from 'lucide-react';
import { FOCUS_RING } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

/** A small info button that explains in a tooltip how a figure is calculated. */
export function InfoTip({ label, text }: { label: string; text: string }): ReactNode {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" aria-label={`Cómo se calcula: ${label}`} className={cn('inline-flex rounded-sm text-fg-3 hover:text-fg-1', FOCUS_RING)}>
          <Info aria-hidden className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-xs">
        {text}
      </TooltipContent>
    </Tooltip>
  );
}
