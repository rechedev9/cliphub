'use client';

import { useState, type ReactNode } from 'react';
import { TEAM_LOGO_NAME_MAX, teamLogoUrl } from '@/lib/api/game-assets';
import { cn } from '@/lib/utils';

function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter((word) => word !== '');
  const letters = words.length > 1 ? words.slice(0, 2).map((word) => word[0]) : [...(words[0] ?? '?')].slice(0, 2);
  return letters.join('').toLocaleUpperCase('es');
}

/**
 * A team's logo from the Major stickers in the user's CS2 install, or its initials
 * in a tile of the same size when the team has none. Decorative: the name sits beside it.
 */
export function TeamLogo({ name, className }: { name: string; className?: string }): ReactNode {
  const [missingFor, setMissingFor] = useState<string | null>(null);
  const missing = missingFor === name;
  const tile = cn('inline-flex size-5 shrink-0 items-center justify-center', className);
  if (missing || name === '' || [...name].length > TEAM_LOGO_NAME_MAX) {
    return (
      <span aria-hidden className={cn(tile, 'rounded-sm bg-surface-4 text-[0.6rem] leading-none font-bold text-fg-2')}>
        {initials(name)}
      </span>
    );
  }
  return (
    <span aria-hidden className={tile}>
      <img src={teamLogoUrl(name)} alt="" loading="lazy" decoding="async" onError={() => setMissingFor(name)} className="size-full object-contain" />
    </span>
  );
}
