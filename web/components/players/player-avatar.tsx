'use client';

import { useState, type ReactNode } from 'react';
import { faceitAvatarSrc, hasFaceitAvatar, playerInitial } from '@/lib/api/faceit-avatar';
import { cn } from '@/lib/utils';

export function PlayerAvatar({ nickname, playerID, avatar, size = 40 }: {
  nickname: string;
  playerID: string;
  /** The record's avatar URL; without one the proxy has nothing to serve, so no request is made. */
  avatar?: string;
  size?: number;
}): ReactNode {
  const [failedSource, setFailedSource] = useState<string | null>(null);
  const src = faceitAvatarSrc(playerID);
  const className = 'shrink-0 rounded-full border border-border bg-surface-3';

  if (hasFaceitAvatar({ avatar }) && failedSource !== src) {
    return <img src={src} alt="" width={size} height={size} onError={() => setFailedSource(src)}
      className={cn(className, 'object-cover')} style={{ width: size, height: size }} />;
  }
  return (
    <span aria-hidden className={cn(className, 'grid place-items-center font-display font-semibold text-fg-2')}
      style={{ width: size, height: size, fontSize: size * 0.4 }}>
      {playerInitial(nickname)}
    </span>
  );
}
