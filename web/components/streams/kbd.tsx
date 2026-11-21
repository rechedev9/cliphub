import type { ReactNode } from 'react';

export function Kbd({ children }: { children: ReactNode }): ReactNode {
  return (
    <kbd aria-hidden className="rounded border border-border-subtle px-1 font-sans text-meta font-normal text-fg-3">
      {children}
    </kbd>
  );
}
