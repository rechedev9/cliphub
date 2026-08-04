'use client';

import type { ReactNode } from 'react';
import { FileText } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { VerdictBadge } from '@/components/cheaters/verdict-badge';
import { isReviewable, type AnticheatMetric, type AnticheatPlayer } from '@/lib/api/anticheat';
import { cn } from '@/lib/utils';

/** Formats a metric value the same way the CLI and the dossier do. */
function formatValue(metric: AnticheatMetric): string {
  switch (metric.unit) {
    case '%':
      return `${metric.value.toFixed(1)} %`;
    case 'ms':
    case '°/s':
      return `${Math.round(metric.value)} ${metric.unit}`;
    default:
      return metric.value.toFixed(2);
  }
}

function formatBaseline(metric: AnticheatMetric): string {
  const { mean, stddev } = metric.baseline;
  switch (metric.unit) {
    case '%':
      return `${mean.toFixed(1)} ± ${stddev.toFixed(1)} %`;
    case 'ms':
    case '°/s':
      return `${Math.round(mean)} ± ${Math.round(stddev)} ${metric.unit}`;
    default:
      return `${mean.toFixed(2)} ± ${stddev.toFixed(2)}`;
  }
}

/** Bar colour by suspicion: alarm only once a metric clears the pro spread. */
function suspicionBarClass(suspicion: number): string {
  if (suspicion >= 80) return 'bg-destructive';
  if (suspicion >= 50) return 'bg-stream';
  return 'bg-primary';
}

/** One metric row: value, reference, and a bar for its 0..100 suspicion. */
function MetricRow({ metric }: { metric: AnticheatMetric }) {
  return (
    <li className="flex flex-col gap-1.5 border-t border-border/60 py-3 first:border-t-0">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <span className="text-sm font-medium text-foreground">{metric.label}</span>
        <span className="font-[family-name:var(--font-mono)] text-xs text-muted-foreground">
          {formatValue(metric)} <span className="opacity-60">vs {formatBaseline(metric)}</span>
        </span>
      </div>
      <p className="text-xs leading-5 text-muted-foreground">{metric.description}</p>
      {metric.applied ? (
        <div className="flex items-center gap-3">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted/60">
            <div
              className={cn('h-full rounded-full transition-[width]', suspicionBarClass(metric.suspicion))}
              style={{ width: `${Math.max(2, Math.min(100, metric.suspicion))}%` }}
            />
          </div>
          <span className="w-24 shrink-0 text-right font-[family-name:var(--font-mono)] text-[11px] text-muted-foreground">
            z {metric.z >= 0 ? '+' : ''}
            {metric.z.toFixed(2)}
          </span>
        </div>
      ) : (
        <span className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.1em] text-muted-foreground">
          muestra insuficiente ({metric.samples})
        </span>
      )}
    </li>
  );
}

export type PlayerDetailProps = {
  player: AnticheatPlayer;
  onOpenDossier: (player: AnticheatPlayer) => void;
  dossierPending: boolean;
};

/**
 * The expanded view of one screened player: every metric against the baseline
 * and the exact ticks a reviewer can seek to. The dossier button appears only
 * for the reviewable bands, so preparing a report is never one click away from
 * a clean scoreboard line.
 */
export function PlayerDetail({ player, onOpenDossier, dossierPending }: PlayerDetailProps): ReactNode {
  return (
    <div className="flex flex-col gap-5 border-t border-border/60 bg-background/40 px-4 py-5 sm:px-5">
      <ul className="flex flex-col">
        {player.metrics.map((metric) => (
          <MetricRow key={metric.id} metric={metric} />
        ))}
      </ul>

      <section className="flex flex-col gap-2">
        <h4 className="font-[family-name:var(--font-mono)] text-xs uppercase tracking-[0.16em] text-muted-foreground">
          Momentos revisables
        </h4>
        {player.evidence.length === 0 ? (
          <p className="text-sm text-muted-foreground">No se ha marcado ningún momento concreto.</p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {player.evidence.map((e) => (
              <li key={`${e.tick}-${e.kind}`} className="text-sm leading-6 text-muted-foreground">
                <span className="font-[family-name:var(--font-mono)] text-xs text-foreground">
                  R{e.round} · tick {e.tick}
                </span>{' '}
                — {e.detail}
              </li>
            ))}
          </ul>
        )}
      </section>

      {isReviewable(player.verdict) ? (
        <div className="flex flex-wrap items-center gap-3">
          <Button
            variant="outline"
            onClick={() => onOpenDossier(player)}
            loading={dossierPending}
            loadingText="PREPARANDO EXPEDIENTE…"
            className="font-[family-name:var(--font-display)] tracking-[0.06em]"
          >
            <FileText aria-hidden />
            PREPARAR EXPEDIENTE
          </Button>
          <span className="text-xs text-muted-foreground">
            Reúne la evidencia para que la denuncies tú desde tu cuenta. TickCut no envía nada.
          </span>
        </div>
      ) : null}
    </div>
  );
}

export function PlayerSummaryRow({
  player,
  expanded,
  onToggle,
}: {
  player: AnticheatPlayer;
  expanded: boolean;
  onToggle: () => void;
}): ReactNode {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-expanded={expanded}
      className="flex w-full items-center gap-4 px-4 py-3.5 text-left transition-colors hover:bg-accent/40 sm:px-5"
    >
      <span className="w-12 shrink-0 font-[family-name:var(--font-mono)] text-lg font-semibold tabular-nums text-foreground">
        {player.score.toFixed(0)}
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate font-medium text-foreground">{player.name || player.steamid64}</span>
        <span className="font-[family-name:var(--font-mono)] text-xs text-muted-foreground">
          {player.steamid64} · {player.gun_kills} bajas · confianza {Math.round(player.confidence * 100)} %
        </span>
      </span>
      <VerdictBadge verdict={player.verdict} className="shrink-0" />
    </button>
  );
}
