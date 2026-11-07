'use client';

import { useId, type ReactNode } from 'react';
import Link from 'next/link';
import type { CreativeBriefItem } from '@/lib/reel-brief';
import { formatVideoDuration } from '@/lib/produce/full-demo-order';
import { Button } from '@/components/ui/button';
import { FORMAT_TONE } from '@/components/studio/format-tone';

type Notice = { code: string; message: string; round_id?: string };

type ActionProps = {
  /** Rounds of the saved plan and whether the settings are saved, e.g. "19 rondas · Ajustes guardados"; null before a plan exists. */
  planState: string | null;
  /** What creating does next, read out as a status line under the button. */
  status: string;
  error: string | null;
  backHref: string;
  /** The create button, built by the producer so its action and loading state stay there. */
  cta: ReactNode;
};

export type FullDemoSummaryProps = ActionProps & {
  /** Seconds of the saved plan; null while the plan is missing or out of date, so it is a row, never the headline. */
  durationSeconds: number | null;
  roundCount: number;
  briefItems: readonly CreativeBriefItem[];
  blockers: readonly Notice[];
  warnings: readonly Notice[];
  assetBusy: boolean;
};

function ErrorNotice({ error, className = '' }: { error: string | null; className?: string }): ReactNode {
  return error ? <p role="alert" className={`rounded-md border border-destructive/40 bg-destructive/10 p-3 text-body-sm text-destructive ${className}`}>{error}</p> : null;
}

/** The sticky recap beside the settings: what will be recorded and the button that records it. */
export function FullDemoSummary({ durationSeconds, roundCount, briefItems, planState, status, blockers, warnings, assetBusy, error, backHref, cta }: FullDemoSummaryProps): ReactNode {
  const headingId = useId();
  return <aside aria-labelledby={headingId}
    className={`flex min-w-0 flex-col gap-4 rounded-2xl border p-5 @[64rem]/content:sticky @[64rem]/content:top-4 ${FORMAT_TONE.full.border} ${FORMAT_TONE.full.tint}`}>
    <div className="flex flex-col gap-0.5">
      <span className="text-meta text-fg-2">Rondas del vídeo</span>
      <span className="font-display text-[3.25rem] leading-none font-black tracking-tight text-fg-1 tabular-nums">
        {roundCount > 0 ? String(roundCount) : 'Todas'}
      </span>
    </div>
    <h2 id={headingId} className="text-body font-extrabold text-fg-1">Lo que se va a grabar</h2>
    <dl className="grid grid-cols-[6rem_minmax(0,1fr)] gap-x-3 gap-y-2 text-body-sm">
      {briefItems.map((item) => <div key={item.label} className="contents">
        <dt className="text-fg-3">{item.label}</dt>
        <dd className="min-w-0 font-semibold break-words text-fg-1">{item.value}</dd>
      </div>)}
      <dt className="text-fg-3">Duración</dt>
      <dd className="min-w-0 font-semibold text-fg-1 tabular-nums">{durationSeconds !== null ? `Unos ${formatVideoDuration(durationSeconds)}` : 'Se calcula al crear'}</dd>
    </dl>
    {blockers.map((item, index) => <p key={`${item.code}-${index}`} role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-body-sm text-destructive">{item.message}{item.round_id ? ` (${item.round_id})` : ''}</p>)}
    {warnings.map((item, index) => <p key={`${item.code}-${index}`} className="text-body-sm text-fg-2">{item.message}</p>)}
    {assetBusy ? <p role="status" className="text-body-sm text-fg-2">Subiendo y verificando el archivo…</p> : null}
    {/* Below 64rem the summary sits under the settings, so FullDemoActionBar carries these instead. */}
    <ErrorNotice error={error} className="hidden @[64rem]/content:block" />
    <div className="hidden flex-col gap-2 @[64rem]/content:flex">
      {planState ? <p className="text-body-sm font-semibold text-fg-1">{planState}</p> : null}
      {cta}
      <Button variant="outline" size="sm" asChild><Link href={backHref}>Volver</Link></Button>
      <p role="status" className="text-meta text-fg-2">{status}</p>
    </div>
  </aside>;
}

/** Below 64rem: the create button stays at the bottom of the window while the settings scroll. */
export function FullDemoActionBar({ planState, status, error, backHref, cta }: ActionProps): ReactNode {
  return <div data-slot="produce-actions"
    className={`sticky bottom-0 z-20 mt-4 flex flex-col gap-3 rounded-2xl border bg-surface-1 px-4 py-3 shadow-[var(--elev-4)] @[64rem]/content:hidden ${FORMAT_TONE.full.border}`}>
    <ErrorNotice error={error} />
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <div className="min-w-0 flex-1 basis-56">
        {planState ? <p className="break-words text-body-sm font-semibold text-fg-1">{planState}</p> : null}
        <p role="status" className="break-words text-meta text-fg-2">{status}</p>
      </div>
      <Button variant="outline" size="sm" asChild><Link href={backHref}>Volver</Link></Button>
      <div className="shrink-0">{cta}</div>
    </div>
  </div>;
}
