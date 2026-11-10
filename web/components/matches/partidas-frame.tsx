'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Cloud, Layers, ListVideo, ShieldAlert, Trophy, type LucideIcon } from 'lucide-react';
import { LIBRARY_SERIES_HREF, MATCHES_HREF, PRO_MATCHES_HREF } from '@/lib/clips/routes';
import { PLAYLISTS_HREF } from '@/lib/replay/saved-playlist';
import { cn } from '@/lib/utils';
import { Button, FOCUS_RING } from '@/components/ui/button';

type PartidasTab = { href: string; label: string; icon: LucideIcon };

const TABS: readonly PartidasTab[] = [
  { href: MATCHES_HREF, label: 'Privadas', icon: Cloud },
  { href: PRO_MATCHES_HREF, label: 'Públicas', icon: Trophy },
  { href: PLAYLISTS_HREF, label: 'Playlists', icon: ListVideo },
  { href: '/integridad', label: 'Integridad', icon: ShieldAlert },
];

/** Privadas, Públicas, Playlists and Integridad: four routes read as one screen, with the tab of the open route underlined. */
function PartidasTabs({ importer }: { importer?: ReactNode }): ReactNode {
  const pathname = usePathname();
  return (
    <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2 border-b border-border">
      <nav aria-label="Partidas" className="flex items-end gap-5">
        {TABS.map(({ href, label, icon: Icon }) => {
          const active = pathname === href;
          return (
            <Link
              key={href}
              href={href}
              aria-current={active ? 'page' : undefined}
              className={cn(
                'relative inline-flex h-9 items-center gap-1.5 text-body-sm font-medium whitespace-nowrap text-fg-3 transition-colors hover:text-fg-1',
                FOCUS_RING,
                active && 'font-semibold text-fg-1 after:absolute after:inset-x-0 after:-bottom-px after:h-0.5 after:bg-primary',
              )}
            >
              <Icon aria-hidden className="size-4" />
              {label}
            </Link>
          );
        })}
      </nav>
      <div className="flex items-center gap-2 pb-1">
        <Button asChild variant="outline" size="sm">
          <Link href={LIBRARY_SERIES_HREF}>
            <Layers aria-hidden />
            Series y veto
          </Link>
        </Button>
        {importer}
      </div>
    </div>
  );
}

/**
 * The page frame of the three Partidas tabs: a centred column of about 950px with dark margins,
 * the tabs on top and the screen under them. `importer` is the live demo importer of Privadas.
 */
export function PartidasFrame({ importer, children }: { importer?: ReactNode; children: ReactNode }): ReactNode {
  return (
    <div className="mx-auto flex w-full max-w-[59.375rem] flex-col gap-4">
      <h1 className="sr-only">Partidas</h1>
      <PartidasTabs importer={importer} />
      {children}
    </div>
  );
}
