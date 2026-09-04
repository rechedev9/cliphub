'use client';

import { usePathname } from 'next/navigation';
import type { ReactElement, ReactNode } from 'react';

/**
 * The shell's one route entrance.
 *
 * Before this, `.studio-enter` was pasted per page and per item: Clips y vídeos
 * and Stream clips faded in, Táctica / Anti-cheat / Players / Ajustes snapped,
 * and on the hub the page container, the banner, the lens and every match row
 * each ran their own copy — four overlapping reveals for one navigation. Two
 * different feels on one sidebar reads as unfinished; four stacked reveals read
 * as generated. The shell owns the moment now, and it owns it once.
 *
 * Keyed by pathname, not by the full URL: `?vista=clips` and `?abierta=<id>` on
 * the hub are the same destination, and replaying the reveal on a filter change
 * would be motion with nothing to say. The existing guards in globals.css cover
 * the rest — reduced motion collapses the duration, an inactive window skips the
 * animation outright rather than pausing it on an invisible first frame.
 */
export function RouteFrame({ children }: { children: ReactNode }): ReactElement {
  const pathname = usePathname();

  return (
    // A plain block, deliberately: pages own their own flex/grid roots, and
    // making the frame a flex container would silently re-parent every one of
    // them into a column it did not ask for.
    <div key={pathname} data-slot="route-frame" className="studio-enter">
      {children}
    </div>
  );
}
