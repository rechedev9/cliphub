'use client';

import { useCallback, useEffect, useState, type CSSProperties, type ReactElement, type ReactNode } from 'react';
import { SidebarProvider } from '@/components/ui/sidebar';

/** Below this width the 240px rail squeezes the producers, so the shell starts as icons. */
const AUTO_COLLAPSE_QUERY = '(max-width: 1199px)';

/**
 * Collapses the sidebar to its icon rail on narrow desktop windows without
 * touching the saved preference: the cookie only changes when the user
 * toggles, so widening the window again restores what they chose.
 */
export function ShellSidebarProvider({
  defaultOpen,
  style,
  children,
}: {
  defaultOpen: boolean;
  style?: CSSProperties;
  children: ReactNode;
}): ReactElement {
  const [userOpen, setUserOpen] = useState(defaultOpen);
  const [forcedClosed, setForcedClosed] = useState(false);

  useEffect(() => {
    const query = window.matchMedia(AUTO_COLLAPSE_QUERY);
    const sync = (): void => setForcedClosed(query.matches);
    sync();
    query.addEventListener('change', sync);
    return () => query.removeEventListener('change', sync);
  }, []);

  const onOpenChange = useCallback((open: boolean) => {
    setForcedClosed(false);
    setUserOpen(open);
  }, []);

  return (
    <SidebarProvider open={forcedClosed ? false : userOpen} onOpenChange={onOpenChange} style={style}>
      {children}
    </SidebarProvider>
  );
}
