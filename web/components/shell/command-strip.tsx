'use client';

import { usePathname } from 'next/navigation';
import { useSyncExternalStore, type ReactElement } from 'react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { JobTransport } from '@/components/shell/job-transport';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';
import { NAV_SECTIONS, type NavSection } from '@/lib/nav';

/**
 * The ceiling of the room: one 56px band across the full inset, above the
 * content column, at every width. It deliberately has no `max-w` and its own
 * `px-2 sm:px-3` rather than `--shell-gutter`, so it aligns with the content
 * column on neither edge — a ceiling spans the room, not the rug.
 *
 * It replaces the `md:hidden` header, which was the only chrome above 768px —
 * i.e. on desktop the app had a sidebar, a page, and nothing else: no location
 * cue, no sidebar toggle reachable with a mouse, and no indication that a
 * capture or render was running. The narrow layout is now the same component
 * with fewer slots rather than a separate bar with a different vocabulary.
 *
 * The surface is opaque `--surface-0`, not `backdrop-blur-md`: this is a
 * full-width sticky element, and a backdrop-filter re-reads and two-pass blurs
 * the entire band on every scroll frame.
 */
export function CommandStrip(): ReactElement {
  const pathname = usePathname();
  const section = sectionForPath(pathname);
  const trail = trailForPath(pathname, section);
  const activity = useSyncExternalStore(
    subscribeToShellActivity,
    shellActivitySnapshot,
    serverShellActivitySnapshot,
  );

  return (
    <header className="shell-strip sticky top-0 z-30 flex h-(--shell-strip-height) shrink-0 items-center gap-2 px-2 sm:gap-3 sm:px-3">
      <SidebarTrigger
        className="size-10 shrink-0 text-fg-2 hover:text-fg-1"
        title="Barra lateral · Ctrl/⌘ B"
      />

      <nav aria-label="Ruta actual" className="flex min-w-0 items-baseline gap-2">
        {section === null ? null : (
          <span
            className="font-[family-name:var(--font-mono)] text-meta text-fg-3 tabular-nums"
            aria-hidden
          >
            {`// ${section.number}`}
          </span>
        )}
        <span className="truncate font-[family-name:var(--font-display)] text-label font-semibold tracking-wide text-fg-1 uppercase">
          {section?.label ?? 'TickCut'}
        </span>
        {trail === null ? null : (
          <>
            <span className="shrink-0 text-fg-4" aria-hidden>
              ›
            </span>
            <span className="max-w-[14ch] truncate font-[family-name:var(--font-mono)] text-meta text-fg-3">
              {trail}
            </span>
          </>
        )}
      </nav>

      <div className="ml-auto flex shrink-0 items-center gap-2">
        <JobTransport />
        {activity.capturing ? <CapturePip /> : null}
      </div>
    </header>
  );
}

/**
 * The GPU-contention pip. Magenta because design.md reserves it for REC, and a
 * capture is literally CS2 recording; it is also the visible counterpart of the
 * `data-capture-active` attribute that silences every ambient effect, so the
 * user can see why the shell just went quiet.
 */
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

/**
 * The nested segment, when there is one. The shell has no way to know an
 * entity's name without fetching it, so it shows the identifier the URL already
 * carries rather than inventing a friendlier label for it.
 */
function trailForPath(pathname: string, section: NavSection | null): string | null {
  if (section === null) return null;
  const rest = pathname.slice(section.href.length).replace(/^\/+|\/+$/g, '');
  if (rest === '') return null;
  return decodeURIComponent(rest.split('/')[0] ?? '');
}
