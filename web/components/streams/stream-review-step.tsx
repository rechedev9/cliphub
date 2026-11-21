'use client';

import { useMemo, type ReactNode } from 'react';
import type { StreamEditPlan } from '@/lib/api/streams';
import { streamCreativeBrief } from '@/lib/streams/brief';
import { streamBlockerHint, type StreamStep } from '@/lib/streams/editor';
import type { StreamShort } from '@/lib/streams/groups';
import { ShortAdjustments } from '@/components/streams/moment-adjustments';
import { StreamReviewList } from '@/components/streams/stream-review-list';
import { CreativeBriefList } from '@/components/studio/creative-brief';
import { Button } from '@/components/ui/button';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';

export function StreamReviewStep({
  plan,
  shorts,
  selectedShortId,
  musicLabel,
  blocker,
  rangesIssue,
  canPlay,
  busy,
  onNavigate,
  onSelectShort,
  onPlaySelected,
  onPlanChange,
}: {
  plan: StreamEditPlan;
  shorts: StreamShort[];
  selectedShortId: string | null;
  musicLabel: string;
  blocker: StreamStep | null;
  rangesIssue: string | null;
  canPlay: boolean;
  busy: boolean;
  onNavigate: (step: StreamStep) => void;
  onSelectShort: (short: StreamShort) => void;
  onPlaySelected: () => void;
  onPlanChange: (plan: StreamEditPlan) => void;
}): ReactNode {
  const briefItems = useMemo(() => streamCreativeBrief(plan, musicLabel), [plan, musicLabel]);
  return (
    <>
      <p className="text-body-sm text-fg-2">Revisa cada Short antes de exportar. Se guardará un vídeo por Short: junta momentos para que salgan en el mismo.</p>
      {blocker ? (
        <Button variant="outline" className="h-auto whitespace-normal" onClick={() => onNavigate(blocker)}>
          {rangesIssue || streamBlockerHint(plan)}
        </Button>
      ) : null}
      <StreamReviewList
        plan={plan}
        shorts={shorts}
        selectedShortId={selectedShortId}
        disabled={busy}
        onSelect={onSelectShort}
        onPlanChange={onPlanChange}
      />
      <Button variant="outline" disabled={!canPlay} onClick={onPlaySelected}>
        Ver este Short
      </Button>
      {selectedShortId ? (
        <ShortAdjustments plan={plan} shortId={selectedShortId} disabled={busy} onPlanChange={onPlanChange} />
      ) : null}
      <details className="rounded border border-border-subtle p-3">
        <DisclosureSummary className="text-body-sm">Detalles de la exportación</DisclosureSummary>
        <CreativeBriefList items={briefItems} />
      </details>
    </>
  );
}
