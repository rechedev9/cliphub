'use client';

import Link, { useLinkStatus } from 'next/link';
import { usePathname } from 'next/navigation';
import type { ReactElement } from 'react';
import {
  BookMarked,
  Clapperboard,
  FileText,
  Film,
  Lightbulb,
  MonitorPlay,
  Settings,
  ShieldCheck,
  Shield,
  Smartphone,
  Swords,
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
import { RESULTS_HREF } from '@/lib/clips/routes';
import { NAV_AREA_LABEL, NAV_SECTIONS, sectionForPath, type NavArea, type NavHref, type NavSection } from '@/lib/nav';
import { cn } from '@/lib/utils';

type NavTrailingKind = 'stream' | 'faceit';

const NAV_META: Record<NavHref, { icon: LucideIcon; trailing?: NavTrailingKind }> = {
  '/videos-largos': { icon: MonitorPlay },
  '/shorts': { icon: Smartphone },
  '/streams': { icon: Clapperboard, trailing: 'stream' },
  '/mis-videos': { icon: Film },
  '/ideas': { icon: Lightbulb },
  '/partidas': { icon: Swords },
  '/equipos': { icon: Shield },
  '/jugadores': { icon: Users, trailing: 'faceit' },
  '/stratbook': { icon: BookMarked },
  '/informes': { icon: FileText },
  '/integridad': { icon: ShieldCheck },
  '/settings': { icon: Settings },
};

const LABELLED_AREAS = ['create', 'analyze'] as const;

function sectionsOf(area: NavArea): NavSection[] {
  return NAV_SECTIONS.filter((section) => section.area === area);
}

/** Brand lockup, one group per work area, then settings and the capture pill in the footer. */
export function AppSidebar(): ReactElement {
  const pathname = usePathname();
  const activeHref = sectionForPath(pathname)?.href ?? null;

  return (
    <Sidebar collapsible="icon">
      {/* Same band as the command strip, so the wordmark and the breadcrumb share one baseline. */}
      <SidebarHeader className="h-(--shell-strip-height) justify-center border-b border-sidebar-border p-0 px-3 group-data-[collapsible=icon]:px-2">
        <Link
          href={RESULTS_HREF}
          aria-label="Ir a Mis vídeos"
          className="inline-flex min-h-10 items-center group-data-[collapsible=icon]:justify-center"
        >
          <Wordmark className="group-data-[collapsible=icon]:hidden" />
          <BrandMark className="hidden size-7 group-data-[collapsible=icon]:block" />
        </Link>
      </SidebarHeader>

      <SidebarContent className="gap-0 pt-3">
        {LABELLED_AREAS.map((area) => (
          <SidebarGroup key={area} className="gap-0.5 px-2 pb-4">
            {/* [font-size:…] rather than `text-meta`: tailwind-merge files an unknown `text-*` under text colour. */}
            <SidebarGroupLabel className="h-7 px-2 [font-size:var(--text-meta)] font-semibold tracking-wider text-fg-3 uppercase">
              {NAV_AREA_LABEL[area]}
            </SidebarGroupLabel>
            <SidebarMenu className="gap-px">
              {sectionsOf(area).map((section) => (
                <NavRow key={section.href} section={section} active={section.href === activeHref} />
              ))}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="gap-2 border-t border-sidebar-border p-0 px-2 pt-2 pb-3">
        <SidebarMenu className="gap-px">
          {sectionsOf('chrome').map((section) => (
            <NavRow key={section.href} section={section} active={section.href === activeHref} />
          ))}
        </SidebarMenu>
        <CaptureReadiness />
      </SidebarFooter>

      {/* Without this the icon rail was reachable only through an undocumented Ctrl/Cmd+B. */}
      <SidebarRail />
    </Sidebar>
  );
}

function NavRow({ section, active }: { section: NavSection; active: boolean }): ReactElement {
  const meta = NAV_META[section.href];
  const Icon = meta.icon;

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        asChild
        isActive={active}
        tooltip={section.label}
        className={cn(
          'shell-nav-key h-8 gap-2.5 rounded-md px-2 font-medium text-fg-2',
          'transition-colors duration-(--dur-instant) ease-standard',
          'hover:bg-sidebar-accent hover:text-fg-1',
          'data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-fg-1',
          'group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-0!',
        )}
      >
        <Link href={section.href} aria-current={active ? 'page' : undefined}>
          <Icon
            className={cn('size-4 shrink-0', active ? 'text-primary' : 'text-fg-3')}
            aria-hidden
          />
          <span className="min-w-0 flex-1 truncate group-data-[collapsible=icon]:hidden">{section.label}</span>
          <NavTrailing kind={meta.trailing} />
          <NavPending />
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

function NavTrailing({ kind }: { kind: NavTrailingKind | undefined }): ReactElement | null {
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
      <span className="shrink-0 font-mono [font-size:var(--text-meta)] font-normal tracking-normal text-fg-3 group-data-[collapsible=icon]:hidden">
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
