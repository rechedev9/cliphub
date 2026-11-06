'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { FilePlus2 } from 'lucide-react';
import { getLibraryTeamScout } from '@/lib/api/library';
import type { ScoutReport } from '@/lib/api/scout';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { newReportHref } from '@/lib/clips/routes';
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
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The sample the answer stands on, or null while it is read. */
  onCounts: (counts: ScopeCounts | null) => void;
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
    getLibraryTeamScout(teamId, {
      maps: map === '' ? [] : [map],
      side,
      opponent_ids: opponent === '' ? [] : [opponent],
      from,
      to,
    })
      .then((report) => {
        if (!active) return;
        setScout(report);
        onCounts({ maps: report.matches.length, rounds: report.tendencies.round_count });
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, opponent, from, to, onCounts]);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button asChild size="sm">
          <Link href={newReportHref(teamId)}>
            <FilePlus2 aria-hidden />
            Crear informe
          </Link>
        </Button>
      </div>
      <ScoutResult failed={failed} scout={scout} teamId={teamId} />
    </div>
  );
}

function ScoutResult({ failed, scout, teamId }: { failed: boolean; scout: ScoutReport | null; teamId: string }): ReactNode {
  const [video, setVideo] = useState<VideoSource[] | null>(null);
  if (failed) {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo leer el scouting de este equipo. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (scout === null) {
    return (
      <div role="status" aria-label="Leyendo las partidas" className="flex flex-col gap-3">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  return (
    <>
      <ScoutView scout={scout} onRowVideo={(evidence) => setVideo(groupEvidenceByMatch(evidence))} />
      {video ? <AnalysisVideoDialog title="Crear vídeo de estas rondas" sources={video} teamId={teamId} onClose={() => setVideo(null)} /> : null}
    </>
  );
}
