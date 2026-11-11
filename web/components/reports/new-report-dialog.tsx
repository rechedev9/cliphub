'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { listLibraryMatches, listLibraryTeams, type LibraryMatch, type LibraryTeam } from '@/lib/api/library';
import type { ReportSide } from '@/lib/api/scout';
import { prettyMapName } from '@/lib/format';
import { REPORT_SIDE_LABEL } from '@/lib/reports/scout-sections';
import { Button } from '@/components/ui/button';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { EMPTY_PICKER_FILTERS, ReportMatchPicker, filterPickerMatches, type PickerFilters } from './report-match-picker';

/** The most matches one report reads (`library.MaxReportMatches`). */
export const MAX_REPORT_MATCHES = 50;

const SIDE_OPTIONS: readonly { value: string; side: ReportSide }[] = [
  { value: 'both', side: '' },
  { value: 'CT', side: 'CT' },
  { value: 'T', side: 'T' },
];

export function sideFromOption(value: string): ReportSide {
  return SIDE_OPTIONS.find((option) => option.value === value)?.side ?? '';
}

export function optionFromSide(side: ReportSide): string {
  return SIDE_OPTIONS.find((option) => option.side === side)?.value ?? 'both';
}

/** The side selector both report dialogs use. */
export function ReportSideSelect({ value, onChange }: { value: ReportSide; onChange: (side: ReportSide) => void }): ReactNode {
  return (
    <Select value={optionFromSide(value)} onValueChange={(next) => onChange(sideFromOption(next))}>
      <SelectTrigger aria-label="Lado" className="h-9 w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {SIDE_OPTIONS.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {REPORT_SIDE_LABEL[option.side]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** What the dialog hands over: the page creates the report, so its row can read "Generando" meanwhile. */
export type ReportDraft = { title: string; team_id: string; team_name: string; match_ids: string[]; side: ReportSide };

/** The matches of the checked maps, newest first, capped at what one report reads. */
function matchesOfMaps(matches: readonly LibraryMatch[], teamId: string, maps: ReadonlySet<string>): string[] {
  return filterPickerMatches(matches, teamId, EMPTY_PICKER_FILTERS, Date.now())
    .filter((match) => maps.has(match.map))
    .slice(0, MAX_REPORT_MATCHES)
    .map((match) => match.id);
}

/** The small "Generar informe" modal: the rival and the maps; title, side and the exact matches sit under Opciones. */
export function NewReportDialog({
  open,
  onOpenChange,
  initialTeamId,
  onGenerate,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialTeamId?: string;
  onGenerate: (draft: ReportDraft) => void;
}): ReactNode {
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  const [teamId, setTeamId] = useState('');
  const [matches, setMatches] = useState<LibraryMatch[] | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [maps, setMaps] = useState<Set<string>>(new Set());
  const [filters, setFilters] = useState<PickerFilters>(EMPTY_PICKER_FILTERS);
  const [title, setTitle] = useState('');
  const [titleEdited, setTitleEdited] = useState(false);
  const [side, setSide] = useState<ReportSide>('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    setFilters(EMPTY_PICKER_FILTERS);
    setSide('');
    setTitleEdited(false);
    setTeamId(initialTeamId ?? '');
    listLibraryTeams()
      .then((list) => setTeams(list.filter((team) => team.match_count > 0).sort((a, b) => b.match_count - a.match_count || a.name.localeCompare(b.name, 'es'))))
      .catch(() => setTeams([]));
  }, [open, initialTeamId]);

  useEffect(() => {
    if (!open || teamId === '') {
      setMatches(null);
      return;
    }
    let active = true;
    setMatches(null);
    listLibraryMatches({ team_id: teamId })
      .then((list) => {
        if (!active) return;
        setMatches(list);
        const everyMap = new Set(list.map((match) => match.map));
        setMaps(everyMap);
        setSelected(new Set(matchesOfMaps(list, teamId, everyMap)));
      })
      .catch(() => {
        if (active) setMatches([]);
      });
    return () => {
      active = false;
    };
  }, [open, teamId]);

  const team = teams?.find((entry) => entry.id === teamId);
  useEffect(() => {
    if (!titleEdited && team) setTitle(`Scouting de ${team.name}`);
  }, [team, titleEdited]);

  const availableMaps = [...new Set((matches ?? []).map((match) => match.map))].sort();

  function toggleMap(map: string): void {
    const next = new Set(maps);
    if (next.has(map)) next.delete(map);
    else next.add(map);
    setMaps(next);
    setSelected(new Set(matchesOfMaps(matches ?? [], teamId, next)));
  }

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    if (selected.size === 0 || selected.size > MAX_REPORT_MATCHES || team === undefined) {
      setError(`Elige entre 1 y ${MAX_REPORT_MATCHES} partidas.`);
      return;
    }
    const ordered = filterPickerMatches(matches ?? [], teamId, EMPTY_PICKER_FILTERS, Date.now())
      .filter((match) => selected.has(match.id))
      .map((match) => match.id);
    onGenerate({ title: title.trim(), team_id: teamId, team_name: team.name, match_ids: ordered, side });
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle className="flex items-baseline gap-2">
              <span aria-hidden className="text-slash">
                /
              </span>
              Generar informe
            </DialogTitle>
            <DialogDescription className="sr-only">
              Elige el rival y los mapas que lee el informe. Las cifras se calculan en este PC a partir de sus demos.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <span className="text-meta font-medium tracking-wider text-fg-3 uppercase">Equipo</span>
            <Select value={teamId} onValueChange={setTeamId}>
              <SelectTrigger aria-label="Equipo" className="h-9 w-full">
                <SelectValue placeholder={teams === null ? 'Cargando equipos' : 'Elige un equipo'} />
              </SelectTrigger>
              <SelectContent>
                {(teams ?? []).map((entry) => (
                  <SelectItem key={entry.id} value={entry.id}>
                    {entry.name} ({entry.match_count})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {teamId === '' ? (
            <p className="text-body-sm text-fg-2">{teams?.length === 0 ? 'No hay equipos con partidas analizadas.' : 'Elige un equipo para ver sus mapas.'}</p>
          ) : null}
          {teamId !== '' && matches === null ? (
            <p role="status" className="text-body-sm text-fg-2">
              Cargando partidas
            </p>
          ) : null}
          {teamId !== '' && matches !== null ? (
            <fieldset className="flex flex-col gap-1.5">
              <legend className="mb-1.5 text-meta font-medium tracking-wider text-fg-3 uppercase">Mapas</legend>
              <div className="grid grid-cols-2 gap-x-4 gap-y-2">
                {availableMaps.map((map) => (
                  <label key={map} className="flex cursor-pointer items-center gap-2 text-body-sm text-fg-1">
                    <input type="checkbox" className="size-4 accent-primary" checked={maps.has(map)} onChange={() => toggleMap(map)} />
                    {prettyMapName(map)}
                  </label>
                ))}
              </div>
              <p className="mt-1 text-meta text-fg-3">
                {selected.size} {selected.size === 1 ? 'partida elegida' : 'partidas elegidas'} de {matches.length}
              </p>
            </fieldset>
          ) : null}
          {teamId !== '' && matches !== null ? (
            <details className="rounded-md border border-border">
              <DisclosureSummary className="px-3 py-2 text-body-sm font-medium text-fg-2">Opciones</DisclosureSummary>
              <div className="flex flex-col gap-3 border-t border-border p-3">
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
                    Título
                    <Input
                      value={title}
                      maxLength={120}
                      onChange={(event) => {
                        setTitle(event.target.value);
                        setTitleEdited(true);
                      }}
                    />
                  </label>
                  <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
                    Lado
                    <ReportSideSelect value={side} onChange={setSide} />
                  </label>
                </div>
                <ReportMatchPicker
                  teamId={teamId}
                  matches={matches}
                  selected={selected}
                  onSelectedChange={setSelected}
                  filters={filters}
                  onFiltersChange={setFilters}
                />
              </div>
            </details>
          ) : null}
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={teamId === '' || selected.size === 0 || title.trim() === ''}>
              Generar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
