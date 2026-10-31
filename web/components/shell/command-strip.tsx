'use client';

import { usePathname } from 'next/navigation';
import { useSyncExternalStore, type ReactElement } from 'react';
import { CircleHelp } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { openAppTour } from '@/lib/app-tour-state';
import { AppUpdateControl } from '@/components/shell/app-update-control';
import { JobTransport } from '@/components/shell/job-transport';
import { ThemeMenu } from '@/components/shell/theme-menu';
import { useCurrentRouteTitle } from '@/components/shell/route-title';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';
import { LONG_VIDEOS_HREF, RESULTS_HREF, SHORTS_HREF } from '@/lib/clips/routes';
import { sectionForPath, type NavSection } from '@/lib/nav';

const TRAIL = {
  newShort: 'nuevo short',
  newFull: 'vídeo largo',
  publish: 'publicar',
  series: 'serie',
} as const;

/** Series belong to the Shorts journey but live outside /shorts. */
const SERIES_PREFIX = '/series/';

/** Job, series and project ids are not names; the page supplies one via useRouteTitle. */
const OPAQUE_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

// Full-inset ceiling: own padding, no max-w, opaque and flat (no backdrop-blur).
export function CommandStrip(): ReactElement {
  const pathname = usePathname();
  const section = sectionForPath(pathname);
  const routeTitle = useCurrentRouteTitle();
  const trail = routeTitle ?? trailForPath(pathname, section);
  const activity = useSyncExternalStore(
    subscribeToShellActivity,
    shellActivitySnapshot,
    serverShellActivitySnapshot,
  );

  return (
    <header
      data-slot="shell-strip"
      className="shell-strip sticky top-0 z-30 flex h-(--shell-strip-height) shrink-0 items-center gap-2 px-2 sm:gap-3 sm:px-3"
    >
      <SidebarTrigger
        className="size-10 shrink-0 text-fg-2 hover:text-fg-1"
        title="Barra lateral · Ctrl/⌘ B"
      />

      {/* 6px inside the label, 14px across the level break, so the breadcrumb
          reads as two levels. */}
      <nav aria-label="Ruta actual" className="flex min-w-0 items-baseline gap-1.5">
        <span className="truncate text-body-sm font-semibold text-fg-1">
          {section?.label ?? 'ClipHub'}
        </span>
        {trail === null ? null : (
          <>
            <span className="shrink-0 px-1 text-fg-4" aria-hidden>
              ›
            </span>
            <span title={trail} className="max-w-[32ch] truncate text-body-sm text-fg-2">
              {trail}
            </span>
          </>
        )}
      </nav>

      <div className="ml-auto flex shrink-0 items-center gap-2">
        <JobTransport />
        {activity.capturing ? <CapturePip /> : null}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid="app-tour-trigger"
          aria-label="Abrir la guía de ClipHub"
          title="Guía de ClipHub"
          className="text-fg-2 hover:text-fg-1 max-sm:w-10 max-sm:px-0"
          onClick={() => openAppTour(0)}
        >
          <CircleHelp aria-hidden />
          <span className="max-sm:sr-only">Guía</span>
        </Button>
        <ThemeMenu />
        <AppUpdateControl />
      </div>
    </header>
  );
}

// Magenta REC pip: visible counterpart of data-capture-active silencing ambient GPU work.
function CapturePip(): ReactElement {
  return (
    <span
      role="status"
      className="hidden h-9 items-center gap-2 rounded-md border border-stream/45 bg-stream/10 px-2.5 sm:flex"
    >
      <span className="live-pulse size-2 rounded-full bg-stream" aria-hidden />
      <span className="text-meta font-medium text-stream-text">
        Captura
      </span>
    </span>
  );
}

// Screen names for the creation flows and Mis vídeos; other sections show a readable nested segment.
function trailForPath(pathname: string, section: NavSection | null): string | null {
  if (section === null) return null;
  if (pathname.startsWith(SERIES_PREFIX)) return TRAIL.series;
  const rest = pathname.slice(section.href.length).replace(/^\/+|\/+$/g, '');
  if (rest === '') return null;
  const segments = rest.split('/');
  if (section.href === '/streams') return 'Editar stream';
  if (section.href === SHORTS_HREF) return TRAIL.newShort;
  if (section.href === LONG_VIDEOS_HREF) return TRAIL.newFull;
  if (section.href === RESULTS_HREF && segments[1] === 'publicar') return TRAIL.publish;
  const segment = decodeURIComponent(segments[0] ?? '');
  return OPAQUE_ID.test(segment) ? null : segment;
}
