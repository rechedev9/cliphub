'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { listLibraryMatches, listLibraryTeams, type LibraryTeam } from '@/lib/api/library';
import { PERIOD_PRESETS } from '@/lib/analysis/scope';
import { prepHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const ALL = 'all';
const RANGE = 'range';
const DAY_MS = 24 * 60 * 60 * 1000;

type MapOption = { map: string; matches: number };

/** Picks the rival, the map and optionally the period, then opens the printable prep. */
export function PrepDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }): ReactNode {
  const router = useRouter();
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  const [teamId, setTeamId] = useState('');
  const [maps, setMaps] = useState<MapOption[] | null>(null);
  const [map, setMap] = useState('');
  const [period, setPeriod] = useState<string>(ALL);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');

  useEffect(() => {
    if (!open) return;
    setTeamId('');
    setPeriod(ALL);
    listLibraryTeams()
      .then((list) => setTeams(list.filter((team) => team.match_count > 0).sort((a, b) => a.name.localeCompare(b.name, 'es'))))
      .catch(() => setTeams([]));
  }, [open]);

  useEffect(() => {
    setMap('');
    setMaps(null);
    if (teamId === '') return undefined;
    let active = true;
    listLibraryMatches({ team_id: teamId })
      .then((matches) => {
        if (!active) return;
        const counts = new Map<string, number>();
        for (const match of matches) counts.set(match.map, (counts.get(match.map) ?? 0) + 1);
        const options = [...counts].map(([name, n]) => ({ map: name, matches: n })).sort((a, b) => b.matches - a.matches || a.map.localeCompare(b.map));
        setMaps(options);
        setMap(options[0]?.map ?? '');
      })
      .catch(() => {
        if (active) setMaps([]);
      });
    return () => {
      active = false;
    };
  }, [teamId]);

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    if (teamId === '' || map === '') return;
    const preset = PERIOD_PRESETS.find((entry) => entry.value === period);
    let range: { from?: string; to?: string } = {};
    if (preset !== undefined) range = { from: new Date(Date.now() - preset.days * DAY_MS).toISOString().slice(0, 10) };
    else if (period === RANGE) range = { from: from || undefined, to: to || undefined };
    onOpenChange(false);
    router.push(prepHref({ teamId, map, ...range }));
  }

  let mapPlaceholder = 'Elige antes el rival';
  if (teamId !== '' && maps === null) mapPlaceholder = 'Cargando mapas';
  else if (maps?.length === 0) mapPlaceholder = 'Sin partidas';
  else if (maps !== null) mapPlaceholder = 'Elige un mapa';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Preparar rival</DialogTitle>
            <DialogDescription>Una hoja para imprimir con sus compras, su ritmo, dónde se ponen de CT y su playbook en el mapa.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Rival
              <Select value={teamId} onValueChange={setTeamId}>
                <SelectTrigger aria-label="Rival" className="h-9 w-full">
                  <SelectValue placeholder={teams === null ? 'Cargando equipos' : 'Elige un equipo'} />
                </SelectTrigger>
                <SelectContent>
                  {(teams ?? []).map((team) => (
                    <SelectItem key={team.id} value={team.id}>
                      {team.name} ({team.match_count})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Mapa
              <Select value={map} onValueChange={setMap} disabled={!maps || maps.length === 0}>
                <SelectTrigger aria-label="Mapa" className="h-9 w-full">
                  <SelectValue placeholder={mapPlaceholder} />
                </SelectTrigger>
                <SelectContent>
                  {(maps ?? []).map((option) => (
                    <SelectItem key={option.map} value={option.map}>
                      {prettyMapName(option.map)} ({option.matches})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
          </div>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Periodo
            <Select value={period} onValueChange={setPeriod}>
              <SelectTrigger aria-label="Periodo" className="h-9 w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>Todas las fechas</SelectItem>
                {PERIOD_PRESETS.map((preset) => (
                  <SelectItem key={preset.value} value={preset.value}>
                    {preset.label}
                  </SelectItem>
                ))}
                <SelectItem value={RANGE}>Entre dos fechas</SelectItem>
              </SelectContent>
            </Select>
          </label>
          {period === RANGE ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
                Desde
                <Input type="date" value={from} max={to || undefined} onChange={(event) => setFrom(event.target.value)} className="h-9" />
              </label>
              <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
                Hasta
                <Input type="date" value={to} min={from || undefined} onChange={(event) => setTo(event.target.value)} className="h-9" />
              </label>
            </div>
          ) : null}
          {teams?.length === 0 ? <p className="text-body-sm text-fg-2">No hay equipos con partidas en la biblioteca.</p> : null}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={teamId === '' || map === ''}>
              Preparar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
