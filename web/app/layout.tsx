import './globals.css';
import type { Metadata } from 'next';
import type { ReactElement, ReactNode } from 'react';
import { Geist, Geist_Mono } from 'next/font/google';
import { Toaster } from '@/components/ui/sonner';
import { ServiceWorkerCleanup } from '@/components/shell/service-worker-cleanup';
import { StudioThemeProvider } from '@/components/shell/theme-provider';
import { WindowActivityPolicy } from '@/components/shell/window-activity-policy';
import { DesktopPerformance } from '@/components/shell/desktop-performance';
import { DesktopErrors } from '@/components/shell/desktop-errors';
import { WebMCPNavigation } from '@/components/shell/webmcp-navigation';

const geistSans = Geist({
  subsets: ['latin'],
  variable: '--font-geist-sans',
});

const geistMono = Geist_Mono({
  subsets: ['latin'],
  variable: '--font-geist-mono',
});

export const metadata: Metadata = {
  // Segments set their own `title` and land as "Partidas · ClipHub".
  title: { default: 'ClipHub', template: '%s · ClipHub' },
  description: 'Crea vídeos y analiza tus demos de CS2 en tu propio equipo.',
};

const fontVars = `${geistSans.variable} ${geistMono.variable}`;

export default function RootLayout({ children }: { children: ReactNode }): ReactElement {
  return (
    // The next/font variables live on <html> so the --font-* tokens in globals.css resolve at :root.
    // next-themes writes the theme class here before paint, hence the hydration opt-out.
    <html lang="es" className={fontVars} data-window-activity="active" suppressHydrationWarning>
      <body className="bg-background text-foreground antialiased">
        <StudioThemeProvider>
          <ServiceWorkerCleanup />
          <WindowActivityPolicy />
          <DesktopPerformance />
          <DesktopErrors />
          <WebMCPNavigation />
          {children}
          <Toaster />
        </StudioThemeProvider>
      </body>
    </html>
  );
}
