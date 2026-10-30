'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import {
  createLibraryReport,
  listLibraryMatches,
  listLibraryTeams,
  type LibraryMatch,
  type LibraryTeam,
} from '@/lib/api/library';
import type { ReportSide } from '@/lib/api/scout';
import { reportHref } from '@/lib/clips/routes';
import { REPORT_SIDE_LABEL } from '@/lib/reports/scout-sections';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
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

/** Creates a scouting report: the rival, the matches it reads and the side it looks at. */
export function NewReportDialog({
  open,
  onOpenChange,
  initialTeamId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialTeamId?: string;
}): ReactNode {
  const router = useRouter();
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  const [teamId, setTeamId] = useState('');
  const [matches, setMatches] = useState<LibraryMatch[] | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [filters, setFilters] = useState<PickerFilters>(EMPTY_PICKER_FILTERS);
  const [title, setTitle] = useState('');
  const [titleEdited, setTitleEdited] = useState(false);
  const [side, setSide] = useState<ReportSide>('');
  const [busy, setBusy] = useState(false);
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
        const recent = filterPickerMatches(list, teamId, EMPTY_PICKER_FILTERS, Date.now()).slice(0, MAX_REPORT_MATCHES);
        setSelected(new Set(recent.map((match) => match.id)));
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

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (selected.size === 0 || selected.size > MAX_REPORT_MATCHES) {
      setError(`Elige entre 1 y ${MAX_REPORT_MATCHES} partidas.`);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const ordered = filterPickerMatches(matches ?? [], teamId, EMPTY_PICKER_FILTERS, Date.now())
        .filter((match) => selected.has(match.id))
        .map((match) => match.id);
      const report = await createLibraryReport({ title: title.trim(), team_id: teamId, match_ids: ordered, side });
      onOpenChange(false);
      router.push(reportHref(report.id));
    } catch {
      setError('No se pudo crear el informe. Comprueba que las partidas siguen en la biblioteca.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-2xl" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Nuevo informe</DialogTitle>
            <DialogDescription>
              Elige el rival y las partidas que vas a leer. Las cifras se calculan en este PC a partir de sus demos, y cada una enlaza a su ronda.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Rival
              <Select value={teamId} onValueChange={setTeamId}>
                <SelectTrigger aria-label="Rival" className="h-9 w-full">
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
            </label>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Lado
              <ReportSideSelect value={side} onChange={setSide} />
            </label>
          </div>
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
          {teamId === '' ? (
            <p className="text-body-sm text-fg-2">{teams?.length === 0 ? 'No hay equipos con partidas analizadas.' : 'Elige un rival para ver sus partidas.'}</p>
          ) : null}
          {teamId !== '' && matches === null ? (
            <p role="status" className="text-body-sm text-fg-2">Cargando partidas</p>
          ) : null}
          {teamId !== '' && matches !== null ? (
            <ReportMatchPicker
              teamId={teamId}
              matches={matches}
              selected={selected}
              onSelectedChange={setSelected}
              filters={filters}
              onFiltersChange={setFilters}
            />
          ) : null}
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || teamId === '' || selected.size === 0 || title.trim() === ''}>
              Crear informe
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
