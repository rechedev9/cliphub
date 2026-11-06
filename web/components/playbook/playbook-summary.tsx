import type { ReactNode } from 'react';
import Link from 'next/link';
import type { PlaybookRound, TeamPlaybook } from '@/lib/api/playbook';
import type { ScoutSkipReason } from '@/lib/api/scout';
import type { TacticalSide } from '@/lib/api/tactical';
import {
  bucketLabel,
  evidenceRefs,
  playbookCategoryLabel,
  playbookCategoryRule,
  roundRefs,
  tempoBuckets,
  winPct,
} from '@/lib/analysis/playbook';
import { secondsLabel } from '@/lib/analysis/prep';
import { categoryKey, type RatePool } from '@/lib/analysis/library-pool';
import { replayHref } from '@/lib/analysis/replay-link';
import { prettyMapName } from '@/lib/format';
import { SKIP_REASON_LABEL } from '@/lib/reports/scout-sections';
import { siteLabel } from '@/lib/tactical-labels';
import { cn } from '@/lib/utils';
import { BarList, ColumnChart } from './bars';

function sideName(side: TacticalSide): string {
  return side === 'T' ? 'T' : 'CT';
}

/** "Navi · Rush en T · Mirage", the replay title of a set of rounds. */
export function playbookReplayTitle(playbook: Pick<TeamPlaybook, 'team_name' | 'side' | 'map'>, what: string): string {
  return `${playbook.team_name} · ${what} en ${sideName(playbook.side)} · ${prettyMapName(playbook.map)}`;
}

/** One card per category: its share of the side's rounds, the count and the win rate; a card opens its rounds. */
/** Gaps under this many points read as even and get no arrow. */
const EVEN_POINTS = 3;

function Arrow({ value, reference, colored }: { value: number; reference: number | null; colored: boolean }): ReactNode {
  if (reference === null || Math.abs(value - reference) < EVEN_POINTS) return null;
  const up = value > reference;
  let tone = 'text-fg-3';
  if (colored) tone = up ? 'text-outcome-win' : 'text-outcome-loss';
  return (
    <span className={cn('ml-0.5 text-[0.7em]', tone)} title={`Biblioteca ${Math.round(reference)} %`}>
      {up ? '▲' : '▼'}
      <span className="sr-only">{`, biblioteca ${Math.round(reference)} %`}</span>
    </span>
  );
}

export function CategoryGrid({
  playbook,
  compact = false,
  reference = null,
}: {
  playbook: TeamPlaybook;
  compact?: boolean;
  /** Use and success pooled over the library's teams on the same side and map: adds ▲▼ beside each number. */
  reference?: RatePool | null;
}): ReactNode {
  return (
    <ul
      aria-label="Tipos de ronda"
      className="grid grid-cols-[repeat(auto-fit,minmax(9.5rem,1fr))] gap-px overflow-hidden rounded-md border border-border bg-border-subtle print:grid-cols-3"
    >
      {playbook.categories.map((category) => {
        const label = playbookCategoryLabel(category.category);
        const empty = category.rounds === 0;
        const body = (
          <>
            <span className="text-body-sm font-medium text-fg-1">{label}</span>
            <span className={cn('text-title font-semibold tabular-nums', empty ? 'text-fg-3' : 'text-fg-1')}>
              {Math.round(category.share.pct)} %
              <Arrow value={category.share.pct} reference={reference?.pct(categoryKey('used', category.category), 10) ?? null} colored={false} />
            </span>
            <span className="text-meta text-fg-2 tabular-nums">
              {category.rounds} {category.rounds === 1 ? 'ronda' : 'rondas'}
              {empty ? '' : ` · gana ${Math.round(category.win_rate.pct)} %`}
              {empty ? null : (
                <Arrow value={category.win_rate.pct} reference={reference?.pct(categoryKey('won', category.category), 6) ?? null} colored />
              )}
            </span>
            {compact ? null : <span className="text-meta text-fg-3">{playbookCategoryRule(category.category, playbook.side, playbook.thresholds)}</span>}
          </>
        );
        return (
          <li key={category.category} className="bg-surface-2">
            {empty ? (
              <div className="flex h-full flex-col gap-0.5 px-3 py-2">{body}</div>
            ) : (
              <Link
                href={replayHref(evidenceRefs(category.refs), playbookReplayTitle(playbook, label))}
                className="flex h-full flex-col gap-0.5 px-3 py-2 outline-none transition-[background-color] hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-inset focus-visible:ring-ring/50"
              >
                {body}
              </Link>
            )}
          </li>
        );
      })}
    </ul>
  );
}

/** Where the side's rounds went: A, B or mid, each with its share, count and wins. */
export function SiteBars({ playbook }: { playbook: TeamPlaybook }): ReactNode {
  const sites = playbook.sites.filter((site) => site.site !== 'none' || site.rounds > 0);
  if (sites.length === 0) return <p className="text-body-sm text-fg-2">Ninguna ronda llegó a un sitio.</p>;
  return (
    <BarList
      label="Sitios"
      items={sites.map((site) => ({
        id: site.site,
        label: site.site === 'none' ? 'Sin sitio' : siteLabel(site.site),
        value: site.rounds,
        text: `${Math.round(site.share.pct)} % · ${site.rounds}`,
        detail: site.rounds > 0 ? `gana ${Math.round(site.win_rate.pct)} %` : undefined,
        href: site.rounds > 0 ? replayHref(evidenceRefs(site.refs), playbookReplayTitle(playbook, `Rondas en ${siteLabel(site.site)}`)) : undefined,
      }))}
    />
  );
}

/** What the tempo measures on each side. */
export function tempoMeaning(side: TacticalSide): string {
  return side === 'T' ? 'Segundos hasta pisar un sitio o plantar' : 'Segundos hasta el primer duelo';
}

/** The rounds' tempo in buckets; each bar opens its rounds. */
export function TempoChart({ playbook, rounds }: { playbook: TeamPlaybook; rounds: readonly PlaybookRound[] }): ReactNode {
  const size = playbook.tempo.bucket_seconds > 0 ? playbook.tempo.bucket_seconds : playbook.thresholds.tempo_bucket_seconds;
  const buckets = tempoBuckets(rounds, size);
  const timed = buckets.reduce((sum, bucket) => sum + bucket.items.length, 0);
  if (timed === 0) return <p className="text-body-sm text-fg-2">Ninguna ronda tiene ritmo medido.</p>;
  const last = buckets[buckets.length - 1];
  return (
    <div className="flex flex-col gap-2">
      <p className="text-meta text-fg-2">
        {tempoMeaning(playbook.side)}. Mediana {secondsLabel(playbook.tempo.median)}, {timed} rondas.
      </p>
      <ColumnChart
        label={tempoMeaning(playbook.side)}
        axisEnd={last ? `${last.fromSeconds + size} s` : ''}
        items={buckets.map((bucket) => {
          const span = bucketLabel(bucket.fromSeconds, size);
          const won = winPct(bucket.items);
          return {
            id: String(bucket.fromSeconds),
            label: span,
            value: bucket.items.length,
            title: `${span}: ${bucket.items.length} rondas${won === null ? '' : `, gana ${won} %`}`,
            top: bucket.items.length > 0 ? `${Math.round((100 * bucket.items.length) / timed)} %` : undefined,
            href: bucket.items.length > 0 ? replayHref(roundRefs(bucket.items), playbookReplayTitle(playbook, `Ritmo ${span}`)) : undefined,
          };
        })}
      />
    </div>
  );
}

/** The matches the answer left out, with each distinct reason once. */
export function SkippedNote({ skipped }: { skipped: readonly { match_id: string; reason: ScoutSkipReason }[] }): ReactNode {
  if (skipped.length === 0) return null;
  const reasons = [...new Set(skipped.map((entry) => SKIP_REASON_LABEL[entry.reason]))];
  return (
    <p className="text-meta text-fg-3">
      {skipped.length} {skipped.length === 1 ? 'partida no cuenta' : 'partidas no cuentan'}. {reasons.join(' ')}
    </p>
  );
}
