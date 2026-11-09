'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { FilePlus2 } from 'lucide-react';
import { getLibraryTeamScout } from '@/lib/api/library';
import type { ScoutReport } from '@/lib/api/scout';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { newReportHref } from '@/lib/clips/routes';
import { EmptySample } from '@/components/analysis/empty-sample';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { ScoutView } from '@/components/reports/scout-view';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
import { groupEvidenceByMatch, type VideoSource } from '@/lib/library/analysis-video';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';

export const ANY = '__any__';

export function OptionSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
}): ReactNode {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger aria-label={label} className="h-9 min-w-36">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ANY}>{label}: todos</SelectItem>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** A team's scouting over its library matches, read through the page's analysis scope. */
export function TeamScout({
  teamId,
  scope,
  lineup,
  onCounts,
  onReport,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  /** The sample the answer stands on, or null while it is read. */
  onCounts: (counts: ScopeCounts | null) => void;
  /** The report once read, or null while it is read or when it failed. */
  onReport?: (report: ScoutReport | null) => void;
}): ReactNode {
  const [scout, setScout] = useState<ScoutReport | null>(null);
  const [failed, setFailed] = useState(false);
  const [now] = useState(() => Date.now());
  const { map, side, opponent } = scope;
  const { from, to } = scopeDateRange(scope.period, now);

  useEffect(() => {
    let active = true;
    setScout(null);
    setFailed(false);
    onCounts(null);
    onReport?.(null);
    getLibraryTeamScout(teamId, {
      maps: map === '' ? [] : [map],
      side,
      opponent_ids: opponent === '' ? [] : [opponent],
      from,
      to,
      lineup,
    })
      .then((report) => {
        if (!active) return;
        setScout(report);
        onCounts({ maps: report.matches.length, rounds: report.tendencies.round_count });
        onReport?.(report);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, opponent, from, to, lineup, onCounts, onReport]);

  return (
    <ScoutResult
      failed={failed}
      scout={scout}
      teamId={teamId}
      actions={
        <Button asChild size="sm">
          <Link href={newReportHref(teamId)}>
            <FilePlus2 aria-hidden />
            Guardar como informe
          </Link>
        </Button>
      }
    />
  );
}

function ScoutResult({ failed, scout, teamId, actions }: { failed: boolean; scout: ScoutReport | null; teamId: string; actions: ReactNode }): ReactNode {
  const [video, setVideo] = useState<VideoSource[] | null>(null);
  if (failed) {
    return (
      <div className="flex flex-col gap-4">
        <div className="flex justify-end">{actions}</div>
        <p role="alert" className="text-body-sm text-destructive">
          No se pudo leer el scouting de este equipo. Comprueba que ClipHub Studio sigue abierto.
        </p>
      </div>
    );
  }
  if (scout === null) {
    // The tab bar's shape, with the real button: a report can be started before the read ends.
    return (
      <div role="status" aria-label="Leyendo las partidas" className="flex flex-col gap-4">
        <div className="flex items-center gap-3 border-b border-border py-1">
          <Skeleton className="h-6 flex-1" />
          {actions}
        </div>
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  const view = <ScoutView scout={scout} actions={actions} onRowVideo={(evidence) => setVideo(groupEvidenceByMatch(evidence))} />;
  return (
    <>
      {/* An empty report empties every sub-tab, so the lineup's note replaces them all. */}
      {scout.matches.length === 0 ? <EmptySample>{view}</EmptySample> : view}
      {video ? <AnalysisVideoDialog title="Crear vídeo de estas rondas" sources={video} teamId={teamId} onClose={() => setVideo(null)} /> : null}
    </>
  );
}
