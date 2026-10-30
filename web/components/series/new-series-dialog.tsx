'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { createLibrarySeries, listLibraryTeams, type LibraryTeam } from '@/lib/api/library';
import { SERIES_FORMATS, type SeriesFormat } from '@/lib/api/veto';
import { librarySeriesHref } from '@/lib/clips/routes';
import { SERIES_FORMAT_LABEL } from '@/lib/library/veto';
import { Segmented } from '@/components/tactical/spatial/spatial-controls';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const NO_TEAM = '';

/** The two team pickers of a series, shared by the new-series dialog and the series record. */
export function SeriesTeamSelect({
  label,
  value,
  teams,
  exclude,
  onChange,
}: {
  label: string;
  value: string;
  teams: readonly LibraryTeam[];
  exclude?: string;
  onChange: (value: string) => void;
}): ReactNode {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger aria-label={label} className="h-9 min-w-48">
        <SelectValue placeholder={label} />
      </SelectTrigger>
      <SelectContent>
        {teams
          .filter((team) => team.id !== exclude)
          .map((team) => (
            <SelectItem key={team.id} value={team.id}>
              {team.name}
            </SelectItem>
          ))}
      </SelectContent>
    </Select>
  );
}

/** Reads every team for the pickers, by name. */
export function useLibraryTeams(): LibraryTeam[] | null {
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  useEffect(() => {
    let active = true;
    listLibraryTeams()
      .then((list) => {
        if (active) setTeams(list.sort((a, b) => a.name.localeCompare(b.name, 'es')));
      })
      .catch(() => {
        if (active) setTeams([]);
      });
    return () => {
      active = false;
    };
  }, []);
  return teams;
}

/** Creates a series and opens its record, where its maps and veto are entered. */
export function NewSeriesDialog({ onClose, teamId }: { onClose: () => void; teamId?: string }): ReactNode {
  const router = useRouter();
  const teams = useLibraryTeams();
  const [format, setFormat] = useState<SeriesFormat>('bo3');
  const [event, setEvent] = useState('');
  const [teamA, setTeamA] = useState(teamId ?? NO_TEAM);
  const [teamB, setTeamB] = useState(NO_TEAM);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const halfTeams = (teamA === NO_TEAM) !== (teamB === NO_TEAM);

  async function submit(formEvent: FormEvent<HTMLFormElement>): Promise<void> {
    formEvent.preventDefault();
    if (halfTeams) return;
    setBusy(true);
    setError(null);
    try {
      const created = await createLibrarySeries({
        format,
        event,
        team_ids: teamA !== NO_TEAM && teamB !== NO_TEAM ? [teamA, teamB] : [],
      });
      router.push(librarySeriesHref(created.id));
    } catch {
      setError('No se pudo crear la serie.');
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="max-w-lg">
        <form onSubmit={(formEvent) => void submit(formEvent)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Nueva serie</DialogTitle>
            <DialogDescription>Una serie agrupa los mapas de un BO1, BO3 o BO5. En su ficha añades las partidas y el veto si lo conoces.</DialogDescription>
          </DialogHeader>
          <Segmented
            label="Formato"
            value={format}
            options={SERIES_FORMATS.map((value) => ({ value, label: SERIES_FORMAT_LABEL[value] }))}
            onChange={setFormat}
          />
          <label className="flex flex-col gap-1.5 text-meta text-fg-3">
            Evento
            <Input value={event} maxLength={120} onChange={(e) => setEvent(e.target.value)} placeholder="Liga, torneo o scrim" />
          </label>
          {teams === null ? null : (
            <div className="flex flex-wrap gap-2">
              <SeriesTeamSelect label="Equipo A" value={teamA} teams={teams} exclude={teamB} onChange={setTeamA} />
              <SeriesTeamSelect label="Equipo B" value={teamB} teams={teams} exclude={teamA} onChange={setTeamB} />
            </div>
          )}
          {halfTeams ? <p className="text-meta text-warning">Elige los dos equipos o ninguno.</p> : null}
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={halfTeams} loading={busy} loadingText="Creando">
              Crear serie
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
