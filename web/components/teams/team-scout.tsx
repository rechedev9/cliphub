'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { FilePlus2 } from 'lucide-react';
import { getLibraryTeamScout, type LibraryMatch } from '@/lib/api/library';
import type { ReportSide, ScoutReport } from '@/lib/api/scout';
import { newReportHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { DATE_RANGES, rivalOf, teamLabel } from '@/lib/library/partidas';
import { ReportSideSelect } from '@/components/reports/new-report-dialog';
import { ScoutView } from '@/components/reports/scout-view';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
import { groupEvidenceByMatch, type VideoSource } from '@/lib/library/analysis-video';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';

export const ANY = '__any__';
const DAY_MS = 24 * 60 * 60 * 1000;

type ScoutFilters = { map: string; rival: string; range: string; side: ReportSide };

/** The start of a DATE_RANGES window ending now, as RFC 3339, or undefined for every date. */
export function rangeStart(range: string, now: number): string | undefined {
  const window = DATE_RANGES.find((entry) => entry.value === range);
  return window === undefined ? undefined : new Date(now - window.days * DAY_MS).toISOString();
}

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

/** A team's scouting over its library matches, filtered by map, side, rival and date. */
export function TeamScout({ teamId, matches }: { teamId: string; matches: readonly LibraryMatch[] }): ReactNode {
  const [filters, setFilters] = useState<ScoutFilters>({ map: ANY, rival: ANY, range: ANY, side: '' });
  const [scout, setScout] = useState<ScoutReport | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    setScout(null);
    setFailed(false);
    getLibraryTeamScout(teamId, {
      maps: filters.map === ANY ? [] : [filters.map],
      side: filters.side,
      opponent_ids: filters.rival === ANY ? [] : [filters.rival],
      from: filters.range === ANY ? undefined : rangeStart(filters.range, Date.now()),
    })
      .then((report) => {
        if (active) setScout(report);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, filters]);

  const maps = [...new Set(matches.map((match) => match.map))].sort().map((map) => ({ value: map, label: prettyMapName(map) }));
  const rivals = new Map<string, string>();
  for (const match of matches) {
    const rival = rivalOf(match, teamId);
    if (rival?.team_id) rivals.set(rival.team_id, teamLabel(rival));
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <OptionSelect label="Mapa" value={filters.map} options={maps} onChange={(map) => setFilters({ ...filters, map })} />
        <div className="w-36">
          <ReportSideSelect value={filters.side} onChange={(side) => setFilters({ ...filters, side })} />
        </div>
        <OptionSelect
          label="Rival"
          value={filters.rival}
          options={[...rivals].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, 'es'))}
          onChange={(rival) => setFilters({ ...filters, rival })}
        />
        <OptionSelect
          label="Fecha"
          value={filters.range}
          options={DATE_RANGES.map((range) => ({ value: range.value, label: range.label }))}
          onChange={(range) => setFilters({ ...filters, range })}
        />
        <Button asChild size="sm" className="ml-auto">
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
