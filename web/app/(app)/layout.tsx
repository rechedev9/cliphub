import { cookies } from 'next/headers';
import type { CSSProperties, ReactElement, ReactNode } from 'react';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { AppSidebar } from '@/components/shell/app-sidebar';
import { CommandStrip } from '@/components/shell/command-strip';
import { ShellActivityMonitor } from '@/components/shell/shell-activity-monitor';
import { AssistantRail } from '@/components/assistant/assistant-rail';
import { assistantRailStateFromCookie } from '@/components/shell/assistant-rail-state';
import {
  ASSISTANT_RAIL_COOKIE_NAME,
  SIDEBAR_COOKIE_NAME,
} from '@/components/shell/shell-cookies';

/** Typed rather than cast: CSSProperties has no index signature for custom
 *  properties, and `as React.CSSProperties` is exactly the silencing cast
 *  web/CLAUDE.md rules out. */
const SHELL_VARS: CSSProperties & { '--sidebar-width': string } = { '--sidebar-width': '240px' };

/**
 * Authenticated app shell: left wall, ceiling, lit stage, right wall.
 *
 * Both chrome states are read from cookies here rather than discovered after
 * hydration. `sidebar_state` was written on every toggle and read by nothing,
 * so a collapsed rail always came back expanded; the assistant's state was in
 * localStorage behind a `() => false` server snapshot, so the first paint was
 * always the narrow branch and the whole right side reflowed once React caught
 * up. Server-rendered geometry costs two cookie reads and removes both.
 */
export default async function AppLayout({ children }: { children: ReactNode }): Promise<ReactElement> {
  const jar = await cookies();
  const sidebarOpen = jar.get(SIDEBAR_COOKIE_NAME)?.value !== 'false';
  const assistantState = assistantRailStateFromCookie(jar.get(ASSISTANT_RAIL_COOKIE_NAME)?.value);

  return (
    <SidebarProvider defaultOpen={sidebarOpen} style={SHELL_VARS}>
      <ShellActivityMonitor />
      <AppSidebar />
      <SidebarInset>
        <CommandStrip />
        <div className="flex min-h-0 flex-1">
          {/*
            @container/content is the contract the domain layer keys its
            breakpoints to. Every `xl:` rule in a card or row used to be
            evaluated against the viewport while the real box was the viewport
            minus a 240px sidebar minus the assistant — less than half of it —
            which is what produced horizontal overflow at the 1440px width
            design.md names as a validation target.

            mr-auto, not mx-auto: with `mx-auto` the column re-centred inside
            whatever space was left, so the H1's left edge sat at 288px at 1440
            and 384px at 1920. Pinning it to the sidebar edge gives the app one
            optical spine that survives a resize.
          */}
          <main className="@container/content mr-auto w-full min-w-0 max-w-[1440px] flex-1 px-(--shell-gutter) py-10">
            {children}
          </main>
          <AssistantRail defaultState={assistantState} />
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
