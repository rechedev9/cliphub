'use client';

import Link, { useLinkStatus } from 'next/link';
import { usePathname } from 'next/navigation';
import type { ReactElement } from 'react';
import {
  Clapperboard,
  Film,
  Radar,
  Settings,
  ShieldAlert,
  Users,
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
import { CLIPS_HREF } from '@/lib/clips/routes';
import { NAV_SECTIONS, type NavHref, type NavSection } from '@/lib/nav';
import { cn } from '@/lib/utils';

const ANALYSIS_GROUP_LABEL = 'Análisis';

type NavGroup = 'entry' | 'analysis' | 'chrome';

/** `entry` has no heading, `analysis` is the one labelled group, `chrome` docks to the footer. */
const NAV_META: Record<NavHref, { icon: LucideIcon; group: NavGroup; trailing?: 'stream' | 'faceit' }> = {
  '/clips': { icon: Film, group: 'entry' },
  '/streams': { icon: Clapperboard, group: 'entry', trailing: 'stream' },
  '/players': { icon: Users, group: 'entry', trailing: 'faceit' },
  '/tactical': { icon: Radar, group: 'analysis' },
  '/cheaters': { icon: ShieldAlert, group: 'analysis' },
  '/settings': { icon: Settings, group: 'chrome' },
};

const ENTRY_SECTIONS = NAV_SECTIONS.filter((section) => NAV_META[section.href].group === 'entry');
const ANALYSIS_SECTIONS = NAV_SECTIONS.filter((section) => NAV_META[section.href].group === 'analysis');
const CHROME_SECTIONS = NAV_SECTIONS.filter((section) => NAV_META[section.href].group === 'chrome');

/** A nav href is active for its exact page and any nested route under it. */
function isActiveHref(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

/** Brand lockup, numbered nav, Análisis group, settings + capture pill footer. */
export function AppSidebar(): ReactElement {
  const pathname = usePathname();

  return (
    <Sidebar collapsible="icon">
      {/* Same 56px band as the command strip, so the wordmark and the
          breadcrumb share one baseline across the shell. */}
      <SidebarHeader className="h-(--shell-strip-height) justify-center border-b border-sidebar-border p-0 px-4 group-data-[collapsible=icon]:px-2">
        <Link
          href={CLIPS_HREF}
          aria-label="Ir a Clips y vídeos"
          className="inline-flex min-h-10 items-center group-data-[collapsible=icon]:justify-center"
        >
          <Wordmark className="group-data-[collapsible=icon]:hidden" />
          <BrandMark className="hidden size-7 group-data-[collapsible=icon]:block" />
        </Link>
      </SidebarHeader>

      <SidebarContent className="gap-0 pt-5">
        <SidebarGroup className="gap-1.5 p-0 pb-4">
          <SidebarMenu className="gap-0.5">
            {ENTRY_SECTIONS.map((section) => (
              <NavRow key={section.href} section={section} pathname={pathname} />
            ))}
          </SidebarMenu>
        </SidebarGroup>

        <SidebarGroup className="gap-1.5 p-0 pb-4">
          {/* [font-size:…] rather than `text-meta`: tailwind-merge files an
              unknown `text-*` under text-colour (see .shell-nav-key). */}
          <SidebarGroupLabel className="h-auto px-4 pb-0.5 font-[family-name:var(--font-mono)] [font-size:var(--text-meta)] tracking-widest text-fg-3 uppercase">
            {ANALYSIS_GROUP_LABEL}
          </SidebarGroupLabel>
          <SidebarMenu className="gap-0.5">
            {ANALYSIS_SECTIONS.map((section) => (
              <NavRow key={section.href} section={section} pathname={pathname} />
            ))}
          </SidebarMenu>
        </SidebarGroup>
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
          'shell-nav-key h-12 gap-3 rounded-none px-4 font-[family-name:var(--font-display)] font-semibold tracking-wide text-fg-1 uppercase',
          'transition-[width,height,padding,background-color,color,box-shadow] duration-(--dur-fast) ease-standard',
          'hover:bg-sidebar-accent hover:text-fg-1',
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
          <NavTrailing kind={meta.trailing} />
          <NavPending />
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

function NavTrailing({ kind }: { kind: 'stream' | 'faceit' | undefined }): ReactElement | null {
  if (kind === 'stream') {
    return (
      <span
        className="size-1.5 shrink-0 rounded-full bg-stream group-data-[collapsible=icon]:hidden"
        aria-hidden
      />
    );
  }
  if (kind === 'faceit') {
    return (
      <span className="shrink-0 font-[family-name:var(--font-mono)] [font-size:var(--text-meta)] font-normal tracking-wider text-fg-3 group-data-[collapsible=icon]:hidden">
        FACEIT
      </span>
    );
  }
  return null;
}

/** Route-transition feedback so a click is visible before the new page mounts. */
function NavPending(): ReactElement | null {
  const { pending } = useLinkStatus();
  return pending ? <span aria-hidden className="ff-nav-pending" /> : null;
}
