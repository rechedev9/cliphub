'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowLeft, BookMarked, GitMerge, Pencil, PlugZap, Shield } from 'lucide-react';
import {
  getLibraryTeam,
  isLibraryNotFound,
  isLibraryOffline,
  listLibraryMatches,
  listLibraryTeams,
  mergeLibraryTeam,
  patchLibraryTeam,
  type LibraryMatch,
  type LibraryRosterEntry,
  type LibraryTeam,
} from '@/lib/api/library';
import { TEAMS_HREF, matchHref, playerHref, teamHref, teamStratbookHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import {
  OUTCOME_LABEL,
  mapPerformance,
  percentLabel,
  rivalOf,
  teamLabel,
  teamOutcome,
} from '@/lib/library/partidas';
import { FILTER_ANY, SEARCH_FILTER } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { useTableView } from '@/components/records/use-table-view';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
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
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { TeamMapHistory } from './team-map-history';
import { TeamScout } from './team-scout';
import { TeamSeries } from './team-series';
import { TeamVeto } from './team-veto';
import { TeamSpatial } from './team-spatial';

type Loaded = { team: LibraryTeam; matches: LibraryMatch[] };
type LoadState = { kind: 'loading' } | { kind: 'ready'; data: Loaded } | { kind: 'missing' } | { kind: 'offline' };

/** A team's record: roster by date, matches with rival and result, and the record per map. */
export function TeamRecord({ teamId }: { teamId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    Promise.all([getLibraryTeam(teamId), listLibraryMatches({ team_id: teamId })])
      .then(([team, matches]) => {
        if (active) setState({ kind: 'ready', data: { team, matches } });
      })
      .catch((error: unknown) => {
        if (!active) return;
        if (isLibraryOffline(error)) setState({ kind: 'offline' });
        else if (isLibraryNotFound(error)) setState({ kind: 'missing' });
        else setState({ kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [teamId, attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando equipo" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={Shield}
        title="Equipo no encontrado"
        description="Puede que se haya fusionado con otro equipo o que el enlace sea antiguo."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={TEAMS_HREF}>Ver equipos</Link>
          </Button>
        }
      />
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudo leer el equipo. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <TeamRecordView data={state.data} onChanged={reload} />;
}

function TeamRecordView({ data, onChanged }: { data: Loaded; onChanged: () => void }): ReactNode {
  const { team, matches } = data;
  const [editing, setEditing] = useState(false);
  const [merging, setMerging] = useState(false);
  useRouteTitle(team.name);
  useDocumentTitle(`${team.name} · Equipo · ClipHub`);

  const outcomes = matches.map((match) => teamOutcome(match, team.id));
  const wins = outcomes.filter((outcome) => outcome === 'win').length;
  const losses = outcomes.filter((outcome) => outcome === 'loss').length;
  const draws = outcomes.filter((outcome) => outcome === 'draw').length;
  const maps = mapPerformance(matches, team.id);
  const last = matches[0];
  const aliases = team.aliases.filter((alias) => alias.toLowerCase() !== team.name.toLowerCase());

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Equipo"
        icon={Shield}
        title={team.name}
        subtitle={aliases.length > 0 ? `También como ${aliases.join(', ')}` : undefined}
        fields={[
          { label: 'Partidas', value: matches.length },
          { label: 'Victorias', value: `${wins}-${losses}${draws > 0 ? `-${draws}` : ''}` },
          { label: '% victorias', value: percentLabel(wins, matches.length) || 'Sin partidas' },
          { label: 'Mapas', value: maps.length },
          { label: 'Última partida', value: last ? formatShortDate(last.played_at) : 'Sin partidas' },
        ]}
        actions={
          <>
            <Button type="button" variant="outline" size="sm" onClick={() => setEditing(true)}>
              <Pencil aria-hidden />
              Editar
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setMerging(true)}>
              <GitMerge aria-hidden />
              Fusionar
            </Button>
            <Button asChild variant="outline" size="sm">
              <Link href={teamStratbookHref(team.id, true)}>
                <BookMarked aria-hidden />
                Jugadas repetidas
              </Link>
            </Button>
            <Button asChild variant="ghost" size="sm">
              <Link href={TEAMS_HREF}>
                <ArrowLeft aria-hidden />
                Todos los equipos
              </Link>
            </Button>
          </>
        }
      />
      <RecordTabs
        tabs={[
          {
            id: 'plantilla',
            label: 'Plantilla',
            count: team.roster?.length ?? 0,
            content: <TeamRoster roster={team.roster ?? []} />,
          },
          {
            id: 'partidas',
            label: 'Partidas',
            count: matches.length,
            content: <TeamMatches team={team} matches={matches} />,
          },
          {
            id: 'mapas',
            label: 'Mapas',
            count: maps.length,
            content: <TeamMapHistory teamId={team.id} matches={matches} />,
          },
          {
            id: 'series',
            label: 'Series',
            content: <TeamSeries team={team} />,
          },
          {
            id: 'veto',
            label: 'Veto',
            content: <TeamVeto teamId={team.id} />,
          },
          {
            id: 'scouting',
            label: 'Scouting',
            content: <TeamScout teamId={team.id} matches={matches} />,
          },
          {
            id: 'radar',
            label: 'Radar 2D',
            content: <TeamSpatial teamId={team.id} matches={matches} />,
          },
        ]}
      />
      <EditTeamDialog open={editing} onOpenChange={setEditing} team={team} onSaved={onChanged} />
      <MergeTeamDialog open={merging} onOpenChange={setMerging} team={team} />
    </div>
  );
}

const ROSTER_COLUMNS: readonly DataTableColumn<LibraryRosterEntry>[] = [
  { id: 'nick', header: 'Jugador', required: true, cell: (entry) => entry.nick || entry.steamid64, sortValue: (entry) => entry.nick },
  { id: 'matches', header: 'Partidas', align: 'end', cell: (entry) => entry.matches, sortValue: (entry) => entry.matches },
  {
    id: 'first',
    header: 'Primera',
    cell: (entry) => <span className="text-fg-2">{formatShortDate(entry.first_played_at)}</span>,
    sortValue: (entry) => Date.parse(entry.first_played_at) || null,
  },
  {
    id: 'last',
    header: 'Última',
    cell: (entry) => <span className="text-fg-2">{formatShortDate(entry.last_played_at)}</span>,
    sortValue: (entry) => Date.parse(entry.last_played_at) || null,
  },
  { id: 'steamid', header: 'SteamID64', cell: (entry) => <span className="font-mono text-meta text-fg-3">{entry.steamid64}</span> },
];

function TeamRoster({ roster }: { roster: readonly LibraryRosterEntry[] }): ReactNode {
  const view = useTableView('equipo-plantilla', {
    sort: { column: 'last', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  return (
    <DataTable
      label="Plantilla"
      rows={roster}
      columns={ROSTER_COLUMNS}
      rowKey={(entry) => entry.steamid64}
      rowHref={(entry) => playerHref(entry.steamid64)}
      view={view}
      empty={<p className="text-body-sm text-fg-2">Sin jugadores en las partidas de este equipo.</p>}
    />
  );
}

function TeamMatches({ team, matches }: { team: LibraryTeam; matches: readonly LibraryMatch[] }): ReactNode {
  const view = useTableView('equipo-partidas', {
    sort: { column: 'date', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const columns: DataTableColumn<LibraryMatch>[] = [
    {
      id: 'date',
      header: 'Fecha',
      required: true,
      cell: (match) => formatShortDate(match.played_at),
      sortValue: (match) => Date.parse(match.played_at) || null,
    },
    { id: 'map', header: 'Mapa', cell: (match) => prettyMapName(match.map), sortValue: (match) => prettyMapName(match.map) },
    {
      id: 'rival',
      header: 'Rival',
      cell: (match) => teamLabel(rivalOf(match, team.id)),
      sortValue: (match) => teamLabel(rivalOf(match, team.id)),
    },
    {
      id: 'result',
      header: 'Resultado',
      cell: (match) => {
        const outcome = teamOutcome(match, team.id);
        if (outcome === null) return '';
        return (
          <span className={cn(outcome === 'win' && 'text-outcome-win', outcome === 'loss' && 'text-outcome-loss')}>
            {OUTCOME_LABEL[outcome]}
          </span>
        );
      },
      sortValue: (match) => teamOutcome(match, team.id),
    },
    {
      id: 'score',
      header: 'Marcador',
      cell: (match) => {
        const own = match.teams.find((entry) => entry.team_id === team.id);
        const rival = rivalOf(match, team.id);
        return <span className="font-mono">{`${own?.score ?? 0}-${rival?.score ?? 0}`}</span>;
      },
    },
    {
      id: 'side',
      header: 'Empezó',
      cell: (match) => {
        const side = match.teams.find((entry) => entry.team_id === team.id)?.start_side;
        return <span className={cn('font-mono text-meta', side === 'CT' ? 'text-side-ct' : 'text-side-t')}>{side ?? ''}</span>;
      },
    },
  ];
  return (
    <DataTable
      label={`Partidas de ${team.name}`}
      rows={matches}
      columns={columns}
      rowKey={(match) => match.id}
      rowHref={(match) => matchHref(match.id)}
      view={view}
      empty={<p className="text-body-sm text-fg-2">Este equipo aún no tiene partidas.</p>}
    />
  );
}

function EditTeamDialog({
  open,
  onOpenChange,
  team,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  team: LibraryTeam;
  onSaved: () => void;
}): ReactNode {
  const [name, setName] = useState(team.name);
  const [aliases, setAliases] = useState(team.aliases.join(', '));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setName(team.name);
    setAliases(team.aliases.join(', '));
    setError(null);
  }, [open, team.name, team.aliases]);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const list = aliases.split(',').map((alias) => alias.trim()).filter((alias) => alias !== '');
      await patchLibraryTeam(team.id, { name: name.trim(), aliases: list });
      onOpenChange(false);
      onSaved();
    } catch {
      setError('No se pudo guardar. Un alias que ya tiene otro equipo se resuelve fusionando los dos.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Editar equipo</DialogTitle>
            <DialogDescription>
              Los alias son los nombres de clan que resuelven a este equipo. Las partidas asignadas solas con un alias nuevo pasan a él.
            </DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Nombre
            <Input autoFocus value={name} maxLength={120} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Alias, separados por comas
            <Input value={aliases} maxLength={600} onChange={(event) => setAliases(event.target.value)} />
          </label>
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || name.trim() === ''}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function MergeTeamDialog({
  open,
  onOpenChange,
  team,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  team: LibraryTeam;
}): ReactNode {
  const router = useRouter();
  const [teams, setTeams] = useState<LibraryTeam[]>([]);
  const [target, setTarget] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTarget('');
    setError(null);
    listLibraryTeams()
      .then((list) => setTeams(list.filter((entry) => entry.id !== team.id).sort((a, b) => a.name.localeCompare(b.name, 'es'))))
      .catch(() => setTeams([]));
  }, [open, team.id]);

  async function merge(): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      const into = await mergeLibraryTeam(team.id, target);
      onOpenChange(false);
      router.replace(teamHref(into.id));
    } catch {
      setError('No se pudieron fusionar los equipos.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Fusionar {team.name}</DialogTitle>
          <DialogDescription>
            Sus partidas y sus alias pasan al equipo que elijas, y esta ficha desaparece. Sirve para un mismo equipo que aparece con dos nombres.
          </DialogDescription>
        </DialogHeader>
        {teams.length === 0 ? (
          <p className="text-body-sm text-fg-2">No hay otro equipo con el que fusionarlo.</p>
        ) : (
          <Select value={target} onValueChange={setTarget}>
            <SelectTrigger aria-label="Equipo que se queda" className="h-9 w-full">
              <SelectValue placeholder="Elige el equipo que se queda" />
            </SelectTrigger>
            <SelectContent>
              {teams.map((entry) => (
                <SelectItem key={entry.id} value={entry.id}>
                  {entry.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
            Cancelar
          </Button>
          <Button type="button" disabled={busy || target === ''} onClick={() => void merge()}>
            <GitMerge aria-hidden />
            Fusionar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
