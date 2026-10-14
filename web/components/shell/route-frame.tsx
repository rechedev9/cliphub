'use client';

import { usePathname } from 'next/navigation';
import type { ReactElement, ReactNode } from 'react';

export function RouteFrame({ children }: { children: ReactNode }): ReactElement {
  const pathname = usePathname();

  return (
    <div key={pathname} data-slot="route-frame" className="studio-enter">
      {children}
    </div>
  );
}
