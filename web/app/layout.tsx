import './globals.css';
import type { Metadata } from 'next';
import type { ReactElement, ReactNode } from 'react';
import { Chakra_Petch, Share_Tech_Mono } from 'next/font/google';
import { Toaster } from '@/components/ui/sonner';
import { ServiceWorkerCleanup } from '@/components/shell/service-worker-cleanup';
import { ShellCanvas } from '@/components/shell/shell-canvas';
import { StudioAmbient } from '@/components/shell/studio-ambient';
import { WindowActivityPolicy } from '@/components/shell/window-activity-policy';

const chakraPetch = Chakra_Petch({
  subsets: ['latin'],
  weight: ['400', '500', '600', '700'],
  variable: '--font-chakra-petch',
});

const shareTechMono = Share_Tech_Mono({
  subsets: ['latin'],
  weight: '400',
  variable: '--font-share-tech-mono',
});

export const metadata: Metadata = {
  // Every route shared one browser title before this template; segments set
  // their own `title` and land as "Biblioteca · FragForge".
  title: { default: 'FragForge', template: '%s · FragForge' },
  description: 'Forja tus frags de CS2 en reels destacados - capturados en tu propio equipo.',
};

const fontVars = `${chakraPetch.variable} ${shareTechMono.variable}`;

export default function RootLayout({ children }: { children: ReactNode }): ReactElement {
  return (
    // The next/font variable classes live on <html> so the composed
    // --font-sans/--font-mono/--font-display tokens in globals.css resolve at
    // :root (declared on <body> they would compute to guaranteed-invalid at
    // :root and the whole app would silently fall back to system fonts).
    <html lang="es" className={`dark ${fontVars}`} data-window-activity="active">
      <body className="bg-background text-foreground antialiased">
        {/*
          The two backdrop planes live here, not inside the app group, so
          /upload, /bootstrap and the 404 get the same room without opting in.
          Both are `position: fixed; z-index: -1`: they paint above the page
          background and below every in-flow element on every route, which is
          what lets them stay viewport-relative on a 3000px page. Order matters
          — the volumetric field is furthest back, the lattice and floor sit on
          top of it.
        */}
        <StudioAmbient />
        <ShellCanvas />
        <ServiceWorkerCleanup />
        <WindowActivityPolicy />
        {children}
        <Toaster />
      </body>
    </html>
  );
}
