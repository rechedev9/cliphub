'use client';

import Link from 'next/link';
import { usePathname, useSearchParams } from 'next/navigation';
import { useSyncExternalStore, type ReactElement } from 'react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { Button } from '@/components/ui/button';
import { AppUpdateControl } from '@/components/shell/app-update-control';
import { JobTransport } from '@/components/shell/job-transport';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';
import { CLIPS_HREF, NEW_DEMO_HREF, PRODUCE_FORMAT, PRODUCE_QUERY } from '@/lib/clips/routes';
import { NAV_SECTIONS, type NavSection } from '@/lib/nav';

const TRAIL = {
  newMatch: 'nueva partida',
  newShort: 'nuevo short',
  newFull: 'nuevo full pov',
  publish: 'publicar',
} as const;

// Full-inset 56px ceiling: own padding, no max-w, opaque --surface-0 (no backdrop-blur).
export function CommandStrip(): ReactElement {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const section = sectionForPath(pathname);
  const trail = trailForPath(pathname, section, searchParams.get(PRODUCE_QUERY.format));
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
          reads as two levels instead of four equal tokens. */}
      <nav aria-label="Ruta actual" className="flex min-w-0 items-baseline gap-1.5">
        {section === null ? null : (
          <span
            className="font-[family-name:var(--font-mono)] text-meta text-fg-3 tabular-nums"
            aria-hidden
          >
            {`// ${section.number}`}
          </span>
        )}
        <span className="truncate font-[family-name:var(--font-display)] text-label font-semibold tracking-wide text-fg-1 uppercase">
          {section?.label ?? 'ClipHub'}
        </span>
        {trail === null ? null : (
          <>
            <span className="shrink-0 px-1 text-fg-4" aria-hidden>
              ›
            </span>
            <span className="max-w-[16ch] truncate font-[family-name:var(--font-mono)] text-meta text-fg-3">
              {trail}
            </span>
          </>
        )}
      </nav>

      <div className="ml-auto flex shrink-0 items-center gap-2">
        <JobTransport />
        {pathname === CLIPS_HREF ? (
          <Button asChild variant="hero" size="sm" className="neon-notch max-sm:hidden">
            <Link href={NEW_DEMO_HREF}>+ Cargar demo</Link>
          </Button>
        ) : null}
        {activity.capturing ? <CapturePip /> : null}
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
      <span className="neon-pulse size-2 rounded-full bg-stream" aria-hidden />
      <span className="font-[family-name:var(--font-mono)] text-meta tracking-wider text-stream-text uppercase">
        Captura
      </span>
    </span>
  );
}

function sectionForPath(pathname: string): NavSection | null {
  for (const section of NAV_SECTIONS) {
    if (pathname === section.href || pathname.startsWith(`${section.href}/`)) return section;
  }
  return null;
}

// Screen names for the 01 section; other sections show the nested segment as-is.
function trailForPath(pathname: string, section: NavSection | null, format: string | null): string | null {
  if (section === null) return null;
  const rest = pathname.slice(section.href.length).replace(/^\/+|\/+$/g, '');
  if (rest === '') return null;
  const segments = rest.split('/');
  if (section.href === CLIPS_HREF) {
    if (pathname === NEW_DEMO_HREF) return TRAIL.newMatch;
    if (segments[1] === 'nuevo') return format === PRODUCE_FORMAT.full ? TRAIL.newFull : TRAIL.newShort;
    if (segments[1] === 'publicar') return TRAIL.publish;
  }
  return decodeURIComponent(segments[0] ?? '');
}
