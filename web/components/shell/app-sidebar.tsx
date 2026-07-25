'use client';

import Link, { useLinkStatus } from 'next/link';
import { usePathname } from 'next/navigation';
import type { ReactElement } from 'react';
import {
  Clapperboard,
  Compass,
  Crosshair,
  Film,
  Newspaper,
  Radar,
  Settings,
  ShieldAlert,
  UploadCloud,
  type LucideIcon,
} from 'lucide-react';
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from '@/components/ui/sidebar';
import { BrandMark, Wordmark } from '@/components/brand/wordmark';
import { CaptureReadiness } from '@/components/shell/capture-readiness';
import { NAV_SECTIONS, type NavHref, type NavSection } from '@/lib/nav';
import { cn } from '@/lib/utils';

/** The three phases of the Studio flow, in rail order. */
const NAV_GROUPS = [
  { id: 'production', label: 'Producción' },
  { id: 'signal', label: 'Señal' },
  { id: 'output', label: 'Salida' },
] as const;

type NavGroupId = (typeof NAV_GROUPS)[number]['id'];

/**
 * Per-section presentation the shared nav data deliberately leaves out.
 * `stream` remains a category signal, never the global selection colour;
 * `group` replaces the `index === 4` divider, which split the rail after
 * "Noticias" — an odd boundary, and one that silently moved whenever anyone
 * reordered lib/nav.ts. `chrome` sections dock to the footer instead: settings
 * is chrome, not a destination in the numbered flow.
 */
const NAV_META: Record<NavHref, { icon: LucideIcon; group: NavGroupId | 'chrome'; stream?: boolean }> = {
  '/matches': { icon: Crosshair, group: 'production' },
  '/upload': { icon: UploadCloud, group: 'production' },
  '/tactical': { icon: Radar, group: 'production' },
  '/cheaters': { icon: ShieldAlert, group: 'production' },
  '/streams': { icon: Clapperboard, group: 'production', stream: true },
  '/news': { icon: Newspaper, group: 'signal' },
  '/videos': { icon: Film, group: 'output' },
  '/feed': { icon: Compass, group: 'output' },
  '/settings': { icon: Settings, group: 'chrome' },
};

const CHROME_SECTIONS = NAV_SECTIONS.filter((section) => NAV_META[section.href].group === 'chrome');

/** A nav href is active for its exact page and any nested route under it. */
function isActiveHref(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

/**
 * The left wall: brand lockup, the numbered nav grouped by production phase,
 * and a footer holding settings plus the local CAPTURA readiness control. The
 * grouping comes from NAV_META rather than a row index, so inserting a section
 * can never leave a divider on the wrong row. The active row is an extruded key
 * (`.shell-nav-key`) rather than the v3 `shadow-[inset_3px_0_0]` sliver, which
 * followed the row's border radius and so pinched to nothing at both ends.
 */
export function AppSidebar(): ReactElement {
  const pathname = usePathname();

  return (
    <Sidebar collapsible="icon">
      {/* Same 56px band as the command strip, so the wordmark and the
          breadcrumb share one baseline across the shell. */}
      <SidebarHeader className="h-(--shell-strip-height) justify-center border-b border-sidebar-border p-0 px-4 group-data-[collapsible=icon]:px-2">
        <Link
          href="/matches"
          aria-label="Ir a Partidas"
          className="inline-flex min-h-10 items-center group-data-[collapsible=icon]:justify-center"
        >
          <Wordmark className="group-data-[collapsible=icon]:hidden" />
          <BrandMark className="hidden size-7 group-data-[collapsible=icon]:block" />
        </Link>
      </SidebarHeader>

      <SidebarContent className="gap-0 pt-5">
        {NAV_GROUPS.map((group) => (
          <SidebarGroup key={group.id} className="gap-1.5 p-0 pb-4">
            {/* [font-size:…] rather than `text-meta`: tailwind-merge files an
                unknown `text-*` under text-colour, so inside cn() a size step
                and a colour step cannot coexist (see .shell-nav-key). */}
            <SidebarGroupLabel className="h-auto px-4 pb-0.5 font-[family-name:var(--font-mono)] [font-size:var(--text-meta)] tracking-widest text-fg-3 uppercase">
              {group.label}
            </SidebarGroupLabel>
            <SidebarMenu className="gap-0.5">
              {NAV_SECTIONS.filter((section) => NAV_META[section.href].group === group.id).map(
                (section) => (
                  <NavRow key={section.href} section={section} pathname={pathname} />
                ),
              )}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="gap-2 border-t border-sidebar-border p-0 pt-2 pb-4">
        <SidebarMenu className="gap-0.5">
          {CHROME_SECTIONS.map((section) => (
            <NavRow key={section.href} section={section} pathname={pathname} />
          ))}
        </SidebarMenu>
        <CaptureReadiness />
      </SidebarFooter>

      {/* Without this the icon rail was reachable only through an undocumented
          Ctrl/Cmd+B, i.e. it shipped as dead code on desktop. */}
      <SidebarRail />
    </Sidebar>
  );
}

function NavRow({ section, pathname }: { section: NavSection; pathname: string }): ReactElement {
  const meta = NAV_META[section.href];
  const active = isActiveHref(pathname, section.href);
  const Icon = meta.icon;

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        asChild
        isActive={active}
        tooltip={`${section.number} · ${section.label}`}
        className={cn(
          // .shell-nav-key carries the 13px step; see its note in globals.css.
          'shell-nav-key h-12 gap-3 rounded-none px-4 font-[family-name:var(--font-display)] font-semibold tracking-wide text-fg-1 uppercase',
          // One declaration: the cva base sets transition-[width,height,padding]
          // and tailwind-merge keeps only the last transition-property, so the
          // v3 `transition-colors` silently deleted the collapse transition and
          // the rows snapped while the container slid.
          'transition-[width,height,padding,background-color,color,box-shadow] duration-(--dur-fast) ease-standard',
          'hover:bg-sidebar-accent hover:text-fg-1',
          // Neutralize the shadcn active defaults so .shell-nav-key's gradient,
          // bevel and 3px key are what actually show.
          'data-[active=true]:bg-transparent data-[active=true]:font-semibold data-[active=true]:text-primary',
          'group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:size-10! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:rounded-md group-data-[collapsible=icon]:p-0!',
        )}
      >
        <Link href={section.href} aria-current={active ? 'page' : undefined}>
          <Icon className="size-4 shrink-0 group-data-[collapsible=icon]:size-[18px]" aria-hidden />
          <span
            aria-hidden
            className={cn(
              'w-5 shrink-0 font-[family-name:var(--font-mono)] [font-size:var(--text-meta)] tabular-nums group-data-[collapsible=icon]:hidden',
              active ? 'text-primary' : 'text-fg-3',
            )}
          >
            {section.number}
          </span>
          <span className="min-w-0 flex-1 truncate group-data-[collapsible=icon]:hidden">
            {section.label}
          </span>
          {meta.stream ? (
            <span
              className="size-1.5 shrink-0 rounded-full bg-stream group-data-[collapsible=icon]:hidden"
              aria-hidden
            />
          ) : null}
          <NavPending />
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

/**
 * Route-transition feedback, from Next's own `useLinkStatus` (15.3+, zero
 * dependency cost). Every `(app)` page fetches in an effect, so a click used to
 * move the cyan active state instantly and then freeze on the old content until
 * the new page mounted; this is the only thing that says the click landed.
 */
function NavPending(): ReactElement | null {
  const { pending } = useLinkStatus();
  return pending ? <span aria-hidden className="ff-nav-pending" /> : null;
}
