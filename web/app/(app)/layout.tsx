import { cookies } from 'next/headers';
import type { CSSProperties, ReactElement, ReactNode } from 'react';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { AppSidebar } from '@/components/shell/app-sidebar';
import { CommandStrip } from '@/components/shell/command-strip';
import { RouteFrame } from '@/components/shell/route-frame';
import { ShellActivityMonitor } from '@/components/shell/shell-activity-monitor';
import { TelemetryNotice } from '@/components/shell/telemetry-notice';
import { SIDEBAR_COOKIE_NAME } from '@/components/shell/shell-cookies';

/** Typed rather than cast: CSSProperties has no index signature for custom
 *  properties, and `as React.CSSProperties` is exactly the silencing cast
 *  web/CLAUDE.md rules out. */
const SHELL_VARS: CSSProperties & { '--sidebar-width': string } = { '--sidebar-width': '240px' };

/**
 * Authenticated app shell: left wall, ceiling, lit stage.
 *
 * The sidebar's state is read from a cookie here rather than discovered after
 * hydration. `sidebar_state` was written on every toggle and read by nothing,
 * so a collapsed sidebar always came back expanded and the whole shell reflowed
 * once React caught up. Server-rendered geometry costs one cookie read and
 * removes that.
 */
export default async function AppLayout({ children }: { children: ReactNode }): Promise<ReactElement> {
  const jar = await cookies();
  const sidebarOpen = jar.get(SIDEBAR_COOKIE_NAME)?.value !== 'false';

  return (
    <SidebarProvider defaultOpen={sidebarOpen} style={SHELL_VARS}>
      <ShellActivityMonitor />
      <TelemetryNotice />
      <AppSidebar />
      <SidebarInset>
        <CommandStrip />
        {/*
          @container/content is the contract the domain layer keys its
          breakpoints to. The shell is two columns — a 240px sidebar and this
          stage — so the content box is wider than it was, but it is still not
          the viewport: subtract the sidebar, then the 1440px cap once it
          binds, then two --shell-gutter columns. At 1920 that is 1440 − 2×61.44
          = 1317px, not 1920. Every `xl:` rule in a card or row evaluated
          against the viewport instead is what produced horizontal overflow at
          the 1440px width design.md names as a validation target.

          mr-auto, not mx-auto: SidebarInset is a flex column, so the auto
          margin decides the cross-axis position. With `mx-auto` the column
          re-centres in whatever space is left, so past 1680px the H1's left
          edge drifts rightward as the window grows. Pinning it to the sidebar
          edge gives the app one optical spine that survives a resize.
        */}
        <main className="@container/content mr-auto w-full max-w-[1440px] flex-1 px-(--shell-gutter) py-10">
          <RouteFrame>{children}</RouteFrame>
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}
