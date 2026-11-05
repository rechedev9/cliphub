'use client';

import type { ReactNode } from 'react';
import type { MapPreference } from '@/lib/api/veto';
import { preferenceExplanation, preferenceScoreText, preferenceWord } from '@/lib/analysis/preference';
import { FOCUS_RING } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

/** A team's preference for a map as "+42 La busca", with how it is computed on hover or focus. */
export function MapPreferenceScore({ preference, withWord = true }: { preference: MapPreference; withWord?: boolean }): ReactNode {
  const { score } = preference;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          className={cn('inline-flex items-baseline gap-1.5 rounded-sm underline decoration-fg-4 decoration-dotted underline-offset-4', FOCUS_RING)}
        >
          <span className={cn('font-medium tabular-nums', score > 0 && 'text-outcome-win', score < 0 && 'text-outcome-loss', score === 0 && 'text-fg-2')}>
            {preferenceScoreText(score)}
          </span>
          {withWord ? <span className="text-fg-2">{preferenceWord(score)}</span> : null}
        </span>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-xs">
        {preferenceExplanation(preference)}
      </TooltipContent>
    </Tooltip>
  );
}
