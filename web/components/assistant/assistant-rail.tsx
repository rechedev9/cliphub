'use client';

import { useSyncExternalStore, type ReactElement } from 'react';
import { ChevronsLeft, ChevronsRight } from 'lucide-react';
import { AssistantPanel } from '@/components/assistant/assistant-panel';
import { AssistantProvider } from '@/components/assistant/assistant-provider';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import {
  assistantRailSnapshot,
  assistantSheetSnapshot,
  setAssistantRailState,
  setAssistantSheetOpen,
  subscribeToAssistantRail,
  ASSISTANT_WIDE_QUERY,
  type AssistantRailState,
} from '@/components/shell/assistant-rail-state';

/**
 * The right wall. Two dark walls with a lit stage between them, rather than a
 * page column wearing the content plane's background and lattice.
 *
 * Layout is CSS, state is a cookie. The rail's box exists or not purely by
 * media query, so the server always renders the correct geometry; only the
 * *contents* wait for hydration, which is why a hard load no longer reflows the
 * whole right side. Below the threshold the assistant is a drawer opened from
 * the command strip — the floating FAB is gone, and with it the collision with
 * the sticky reel action it used to land on.
 */
export function AssistantRail({ defaultState }: { defaultState: AssistantRailState }): ReactElement {
  const railState = useSyncExternalStore(
    subscribeToAssistantRail,
    assistantRailSnapshot,
    () => defaultState,
  );
  const sheetOpen = useSyncExternalStore(subscribeToAssistantRail, assistantSheetSnapshot, () => false);
  // The panel is heavy, so it mounts only where it is actually visible; the
  // aside keeps its width from the first byte either way.
  const wide = useSyncExternalStore(subscribeToWideLayout, wideLayoutSnapshot, () => false);
  const collapsed = railState === 'collapsed';

  return (
    <AssistantProvider>
      <aside
        aria-label="Asistente"
        data-state={railState}
        className={
          collapsed
            ? 'shell-wall shell-wall-right hidden w-(--assistant-width-collapsed) shrink-0 flex-col items-center gap-3 py-3 rail:flex'
            : 'shell-wall shell-wall-right hidden w-(--assistant-width) shrink-0 flex-col p-3 rail:flex'
        }
      >
        {collapsed ? (
          <div className="sticky top-[calc(var(--shell-strip-height)+0.75rem)] flex flex-col items-center gap-3">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => setAssistantRailState('expanded')}
              aria-label="Mostrar asistente"
              title="Mostrar asistente"
            >
              <ChevronsLeft aria-hidden />
            </Button>
            <span
              aria-hidden
              className="font-[family-name:var(--font-mono)] text-meta tracking-widest text-fg-3 uppercase [writing-mode:vertical-rl]"
            >
              Agente
            </span>
          </div>
        ) : (
          <div className="sticky top-[calc(var(--shell-strip-height)+0.75rem)] flex h-[calc(100svh-var(--shell-strip-height)-1.5rem)] min-h-0 flex-col gap-2">
            <div className="flex shrink-0 items-center justify-between gap-2">
              <span className="font-[family-name:var(--font-mono)] text-meta tracking-widest text-fg-3 uppercase">
                Agente
              </span>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={() => setAssistantRailState('collapsed')}
                aria-label="Ocultar asistente"
                title="Ocultar asistente"
              >
                <ChevronsRight aria-hidden />
              </Button>
            </div>
            {wide ? <AssistantPanel className="min-h-0 min-w-0 flex-1" /> : null}
          </div>
        )}
      </aside>

      <Sheet open={sheetOpen} onOpenChange={setAssistantSheetOpen}>
        <SheetContent side="right" className="w-[min(100vw,27rem)] max-w-none gap-0 p-0 sm:max-w-none">
          <SheetHeader className="sr-only">
            <SheetTitle>Agente de FragForge</SheetTitle>
            <SheetDescription>Agente integrado capaz de operar los flujos de FragForge Studio.</SheetDescription>
          </SheetHeader>
          <AssistantPanel className="h-full min-h-0 border-0" />
        </SheetContent>
      </Sheet>
    </AssistantProvider>
  );
}

function subscribeToWideLayout(onChange: () => void): () => void {
  const media = window.matchMedia(ASSISTANT_WIDE_QUERY);
  media.addEventListener('change', onChange);
  return () => media.removeEventListener('change', onChange);
}

function wideLayoutSnapshot(): boolean {
  return window.matchMedia(ASSISTANT_WIDE_QUERY).matches;
}
