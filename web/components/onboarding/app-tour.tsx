'use client';

import Link from 'next/link';
import { useEffect, useSyncExternalStore, type ReactElement } from 'react';
import { ArrowLeft, ArrowRight, ArrowUpRight, Check } from 'lucide-react';
import { TOUR_CHAPTERS, TOUR_TITLE, type TourChapter } from '@/lib/app-tour';
import {
  appTourSnapshot,
  closeAppTour,
  markTourSeen,
  openAppTour,
  serverAppTourSnapshot,
  setAppTourChapter,
  shouldAutoOpenTour,
  subscribeToAppTour,
  tourWasSeen,
} from '@/lib/app-tour-state';
import { getDesktopSettingsBridge } from '@/lib/desktop-settings';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';

function browserStorage(): Storage | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

/**
 * Studio tour: one layer over the shell that walks every section. Opens by
 * itself once on a fresh install, and again from the command strip's "Guía".
 */
export function AppTour(): ReactElement {
  const state = useSyncExternalStore(subscribeToAppTour, appTourSnapshot, serverAppTourSnapshot);
  const last = TOUR_CHAPTERS.length - 1;
  const index = Math.min(Math.max(state.chapter, 0), last);
  const chapter = TOUR_CHAPTERS[index];

  useEffect(() => {
    const autoOpen = shouldAutoOpenTour({
      desktop: getDesktopSettingsBridge() !== null,
      seen: tourWasSeen(browserStorage()),
      telemetryNotice: state.telemetryNotice,
    });
    if (autoOpen) openAppTour(0);
  }, [state.telemetryNotice]);

  const close = (): void => {
    // Any exit counts: a tour dismissed on chapter two is still a tour seen.
    markTourSeen(browserStorage(), Date.now());
    closeAppTour();
  };

  return (
    <Dialog open={state.open} onOpenChange={(open) => (open ? openAppTour(index) : close())}>
      <DialogContent
        data-testid="app-tour"
        aria-describedby="app-tour-lead"
        className="max-h-[calc(100dvh-2rem)] gap-0 overflow-hidden p-0 sm:max-w-4xl"
      >
        <div className="grid min-h-0 md:grid-cols-[232px_minmax(0,1fr)]">
          <nav
            aria-label="Capítulos de la guía"
            className="hidden border-r border-border bg-surface-3 py-5 md:block"
          >
            <DialogTitle className="px-5 pb-4 font-mono text-meta font-normal uppercase tracking-widest text-fg-3">
              {TOUR_TITLE}
            </DialogTitle>
            <ol className="flex flex-col">
              {TOUR_CHAPTERS.map((entry, i) => (
                <li key={entry.id}>
                  <button
                    type="button"
                    aria-current={i === index ? 'step' : undefined}
                    onClick={() => setAppTourChapter(i)}
                    className={cn(
                      'flex min-h-10 w-full items-center gap-3 px-5 py-2 text-left text-body-sm transition-colors duration-(--dur-fast)',
                      'focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
                      i === index ? 'bg-surface-4 text-primary' : 'text-fg-2 hover:bg-surface-4 hover:text-fg-1',
                    )}
                  >
                    <span
                      aria-hidden
                      className={cn('w-6 shrink-0 font-mono text-meta tabular-nums', i === index ? 'text-primary' : 'text-fg-3')}
                    >
                      {i < index ? <Check className="size-3.5" /> : entry.kicker}
                    </span>
                    <span className="min-w-0 truncate">{entry.label}</span>
                  </button>
                </li>
              ))}
            </ol>
          </nav>

          <ChapterPanel chapter={chapter} index={index} last={last} onClose={close} />
        </div>
      </DialogContent>
    </Dialog>
  );
}

function ChapterPanel({
  chapter,
  index,
  last,
  onClose,
}: {
  chapter: TourChapter;
  index: number;
  last: number;
  onClose: () => void;
}): ReactElement {
  return (
    <div className="flex max-h-[calc(100dvh-2rem)] min-h-0 flex-col md:max-h-[min(640px,calc(100dvh-2rem))]">
      <div className="min-h-0 flex-1 overflow-y-auto px-6 pt-6 pb-5 md:px-8 md:pt-8">
        {/* Below md the chapter rail is hidden, so the dialog title moves here. */}
        <p className="mb-3 font-mono text-meta uppercase tracking-widest text-fg-3 md:hidden">{TOUR_TITLE}</p>
        <div aria-live="polite">
          <p className="font-mono text-meta uppercase tracking-widest text-primary">
            {chapter.kicker} · {chapter.label}
          </p>
          <h3 className="mt-2 pr-8 font-display text-title font-bold text-fg-1">{chapter.title}</h3>
          <DialogDescription id="app-tour-lead" className="mt-2 text-body text-fg-2">
            {chapter.lead}
          </DialogDescription>
          <dl className="mt-5 flex flex-col gap-3.5">
            {chapter.points.map((point) => (
              <div key={point.term} className="border-l-2 border-border-strong pl-3.5">
                <dt className="font-display text-label font-bold uppercase tracking-wide text-fg-1">{point.term}</dt>
                <dd className="mt-0.5 text-body-sm text-fg-2">{point.text}</dd>
              </div>
            ))}
          </dl>
          {chapter.link === undefined ? null : (
            <Button asChild variant="outline-primary" size="sm" className="mt-6">
              <Link href={chapter.link.href} onClick={onClose}>
                {chapter.link.label}
                <ArrowUpRight aria-hidden />
              </Link>
            </Button>
          )}
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2 border-t border-border px-6 py-3.5 md:px-8">
        <span className="mr-auto font-mono text-meta tabular-nums text-fg-3" aria-label={`Paso ${index + 1} de ${last + 1}`}>
          {String(index + 1).padStart(2, '0')} / {String(last + 1).padStart(2, '0')}
        </span>
        {index > 0 ? (
          <Button type="button" variant="ghost" size="sm" onClick={() => setAppTourChapter(index - 1)}>
            <ArrowLeft aria-hidden />
            Anterior
          </Button>
        ) : (
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>
            Saltar guía
          </Button>
        )}
        {index < last ? (
          <Button type="button" size="sm" onClick={() => setAppTourChapter(index + 1)}>
            Siguiente
            <ArrowRight aria-hidden />
          </Button>
        ) : (
          <Button type="button" size="sm" onClick={onClose}>
            Empezar
          </Button>
        )}
      </div>
    </div>
  );
}
