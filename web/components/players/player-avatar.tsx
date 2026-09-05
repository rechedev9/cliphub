'use client';

import { useState, type ReactNode } from 'react';
import { cn } from '@/lib/utils';

export function PlayerAvatar({ nickname, playerID, size = 40 }: {
  nickname: string;
  playerID: string;
  size?: number;
}): ReactNode {
  const [failedSource, setFailedSource] = useState<string | null>(null);
  const src = `/api/faceit/players/${encodeURIComponent(playerID)}/avatar`;
  const className = 'shrink-0 rounded-full border border-border bg-surface-3';

  if (failedSource !== src) {
    return <img src={src} alt="" width={size} height={size} onError={() => setFailedSource(src)}
      className={cn(className, 'object-cover')} style={{ width: size, height: size }} />;
  }
  return (
    <span aria-hidden className={cn(className, 'grid place-items-center font-display font-semibold text-fg-2')}
      style={{ width: size, height: size, fontSize: size * 0.4 }}>
      {nickname.slice(0, 1).toUpperCase() || '?'}
    </span>
  );
}
