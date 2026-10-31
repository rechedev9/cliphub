'use client';

import type { ReactNode } from 'react';
import { ThemeProvider } from 'next-themes';

/** Dark stays the default so the desktop window's first paint and the page agree. */
export function StudioThemeProvider({ children }: { children: ReactNode }): ReactNode {
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem disableTransitionOnChange>
      {children}
    </ThemeProvider>
  );
}
