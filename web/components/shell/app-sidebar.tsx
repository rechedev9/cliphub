'use client';

import Link, { useLinkStatus } from 'next/link';
import { usePathname, useSearchParams } from 'next/navigation';
import type { ReactElement } from 'react';
import {
  BookOpen,
  ChartColumn,
  Clapperboard,
  Crosshair,
  Film,
  HardDrive,
  Lightbulb,
  ListChecks,
  MonitorPlay,
  NotebookPen,
  Play,
  Search,
  Settings,
  Shield,
  Smartphone,
  Swords,
  Trophy,
  Upload,
  UserRound,
  UserSearch,
  Users,
  Vote,
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
import { Button } from '@/components/ui/button';
import { BrandMark, Wordmark } from '@/components/brand/wordmark';
import { CaptureReadiness } from '@/components/shell/capture-readiness';
import { SidebarDemoImport } from '@/components/shell/sidebar-demo-import';
import { currentEdition, homeLabelFor, homePathFor } from '@/lib/edition';
import {
  VOICE_HREF,
  activeEntryId,
  sidebarFooterEntriesFor,
  sidebarGroupsFor,
  type SidebarEntry,
  type SidebarEntryId,
} from '@/lib/nav-groups';
import { sidebarHref, type PageLocation } from '@/lib/nav-scope';
import { cn } from '@/lib/utils';

// The Crear formats tint a small dot with the colour their flow uses; the icon stays neutral.
const ENTRY_META: Record<SidebarEntryId, { icon: LucideIcon; dot?: string }> = {
  'crear-videos': { icon: Clapperboard },
  'demo-completa': { icon: MonitorPlay, dot: 'bg-format-full' },
  short: { icon: Smartphone, dot: 'bg-format-short' },
  'clips-de-stream': { icon: Clapperboard, dot: 'bg-format-stream' },
  'jugadores-faceit': { icon: UserSearch },
  'mis-videos': { icon: Film },
  ideas: { icon: Lightbulb },
  partidas: { icon: Swords },
  replay: { icon: Play },
  'buscador-de-patrones': { icon: Search },
  'estadisticas-de-equipo': { icon: Shield },
  'estadisticas-de-jugador': { icon: ChartColumn },
  ranking: { icon: Trophy },
  'editor-de-tacticas': { icon: NotebookPen },
  playbook: { icon: BookOpen },
  'lineups-pro': { icon: Crosshair },
  'simulacion-de-veto': { icon: Vote },
  'preparar-rival': { icon: ListChecks },
  almacenamiento: { icon: HardDrive },
  ajustes: { icon: Settings },
};

/** Brand lockup, the EDGE rail groups with slash labels, then Demo, Voz, settings and the capture pill in the footer. */
export function AppSidebar(): ReactElement {
  const pathname = usePathname();
  const params = useSearchParams();
  const edition = currentEdition();
  const groups = sidebarGroupsFor(edition);
  const footerEntries = sidebarFooterEntriesFor(edition);
  const activeId = activeEntryId({ pathname, tab: params.get('tab') });
  const where: PageLocation = { pathname, search: params.toString() };

  return (
    <Sidebar collapsible="icon">
      {/* Same band as the command strip, so the wordmark and the breadcrumb share one baseline. */}
      <SidebarHeader className="h-(--shell-strip-height) justify-center border-b border-sidebar-border p-0 px-3 group-data-[collapsible=icon]:px-2">
        <Link
          href={homePathFor(edition)}
          aria-label={`Ir a ${homeLabelFor(edition)}`}
          className="inline-flex min-h-8 items-center group-data-[collapsible=icon]:justify-center"
        >
          <Wordmark className="group-data-[collapsible=icon]:hidden" />
          <BrandMark className="hidden size-7 group-data-[collapsible=icon]:block" />
        </Link>
      </SidebarHeader>

      <SidebarContent className="gap-0 pt-2">
        {groups.map((group) => (
          <SidebarGroup key={group.id} className="gap-0.5 px-2 pt-1 pb-3">
            {/* [font-size:…] rather than `text-meta`: tailwind-merge files an unknown `text-*` under text colour. */}
            <SidebarGroupLabel className="h-6 gap-1.5 px-2 [font-size:var(--text-meta)] font-semibold tracking-widest text-fg-3 uppercase">
              <span aria-hidden className="text-slash">
                /
              </span>
              <span className="truncate whitespace-nowrap">{group.label}</span>
              <span aria-hidden className="ml-1 h-px min-w-0 flex-1 bg-border" />
              <span aria-hidden className="h-px w-1.5 bg-fg-4" />
            </SidebarGroupLabel>
            <SidebarMenu className="gap-px">
              {group.entries.map((entry) => (
                <NavRow key={entry.id} entry={entry} href={sidebarHref(entry, where)} active={entry.id === activeId} />
              ))}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="gap-2 border-t border-sidebar-border p-0 px-2 pt-2 pb-2">
        {edition === 'web' ? <WebFooterActions pathname={pathname} /> : (
          <div className="flex gap-1.5 group-data-[collapsible=icon]:flex-col group-data-[collapsible=icon]:items-center">
            <SidebarDemoImport />
            <Button
              asChild
              variant="outline"
              size="sm"
              title="Importar voz del equipo"
              className="min-w-0 flex-1 group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:flex-none group-data-[collapsible=icon]:px-0"
            >
              <Link href={VOICE_HREF}>
                <Upload aria-hidden className="stroke-[1.5]" />
                <span className="group-data-[collapsible=icon]:sr-only">Voz</span>
              </Link>
            </Button>
          </div>
        )}
        <SidebarMenu className="gap-px">
          {footerEntries.map((entry) => (
            <NavRow key={entry.id} entry={entry} href={sidebarHref(entry, where)} active={entry.id === activeId} />
          ))}
        </SidebarMenu>
        {/* The web has no PC that records, so no capture status. */}
        {edition === 'web' ? null : <CaptureReadiness />}
      </SidebarFooter>

      {/* Without this the icon rail was reachable only through an undocumented Ctrl/Cmd+B. */}
      <SidebarRail />
    </Sidebar>
  );
}

/** The web edition's footer: import goes to the browser scanner, and the account and team live here. */
function WebFooterActions({ pathname }: { pathname: string }): ReactElement {
  const links = [
    { href: '/equipo', label: 'Equipo', icon: Users },
    { href: '/cuenta', label: 'Cuenta', icon: UserRound },
  ] as const;
  return (
    <>
      <Button asChild variant="outline" size="sm" title="Importar demos" className="group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:px-0">
        <Link href="/importar">
          <Upload aria-hidden className="stroke-[1.5]" />
          <span className="group-data-[collapsible=icon]:sr-only">Importar demos</span>
        </Link>
      </Button>
      <SidebarMenu className="gap-px">
        {links.map(({ href, label, icon: Icon }) => (
          <SidebarMenuItem key={href}>
            <SidebarMenuButton asChild isActive={pathname === href} tooltip={label} className="h-7 gap-2.5 rounded-md px-2 font-medium text-fg-2">
              <Link href={href}>
                <Icon aria-hidden className="size-4 shrink-0 stroke-[1.5]" />
                <span>{label}</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        ))}
      </SidebarMenu>
    </>
  );
}

function NavRow({ entry, href, active }: { entry: SidebarEntry; href: string; active: boolean }): ReactElement {
  const meta = ENTRY_META[entry.id];
  const Icon = meta.icon;

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        asChild
        isActive={active}
        tooltip={entry.label}
        className={cn(
          'shell-nav-key h-7 gap-2.5 rounded-md px-2 font-medium text-fg-2',
          'transition-colors duration-(--dur-instant) ease-standard',
          'hover:bg-sidebar-accent hover:text-fg-1',
          'data-[active=true]:bg-sidebar-accent data-[active=true]:font-semibold data-[active=true]:text-fg-1',
          'group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-0!',
        )}
      >
        <Link href={href} aria-current={active ? 'page' : undefined}>
          <Icon className={cn('size-4 shrink-0 stroke-[1.5]', active ? 'text-fg-1' : 'text-fg-3')} aria-hidden />
          {/* sr-only, not hidden: the icon rail keeps the link's name for screen readers. */}
          <span className="min-w-0 flex-1 truncate group-data-[collapsible=icon]:sr-only">{entry.label}</span>
          {meta.dot === undefined ? null : (
            <span aria-hidden className={cn('size-1.5 shrink-0 rounded-full group-data-[collapsible=icon]:hidden', meta.dot)} />
          )}
          <NavPending />
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

/** Route-transition feedback so a click is visible before the new page mounts. */
function NavPending(): ReactElement | null {
  const { pending } = useLinkStatus();
  return pending ? <span aria-hidden className="ff-nav-pending" /> : null;
}
