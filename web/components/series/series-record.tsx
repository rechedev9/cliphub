'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Layers, Plus, PlugZap, Trash2, Wand2, X } from 'lucide-react';
import {
  deleteLibrarySeries,
  getLibrarySeries,
  isLibraryNotFound,
  isLibraryOffline,
  listLibraryMatches,
  patchLibraryMatch,
  patchLibrarySeries,
  type LibraryMatch,
} from '@/lib/api/library';
import {
  DEFAULT_MAP_POOL,
  EXTRA_POOL_MAPS,
  SERIES_FORMATS,
  VETO_ACTIONS,
  type LibrarySeries,
  type SeriesFormat,
  type VetoAction,
  type VetoStep,
  type VetoTeam,
} from '@/lib/api/veto';
import { LIBRARY_SERIES_HREF, matchHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { matchScoreLabel, matchTitle } from '@/lib/library/partidas';
import { SERIES_FORMAT_LABEL, VETO_ACTION_LABEL, seriesVetoTemplate, vetoStepsProblem } from '@/lib/library/veto';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { RelatedList } from '@/components/records/related-list';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { Segmented } from '@/components/tactical/spatial/spatial-controls';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { SeriesTeamSelect, useLibraryTeams } from './new-series-dialog';
import { seriesName } from './series-library';

type Loaded = { series: LibrarySeries; matches: LibraryMatch[]; candidates: LibraryMatch[] };
type LoadState = { kind: 'loading' } | { kind: 'ready'; data: Loaded } | { kind: 'missing' } | { kind: 'offline' };

const NO_TEAM = '';
const NO_MATCH = '';

/** Matches that can join the series: those of its teams, or every match, that are in no series yet. */
async function seriesCandidates(series: LibrarySeries): Promise<LibraryMatch[]> {
  const lists = series.team_ids.length > 0
    ? await Promise.all(series.team_ids.map((id) => listLibraryMatches({ team_id: id })))
    : [await listLibraryMatches()];
  const seen = new Set<string>();
  return lists.flat().filter((m) => {
    if (m.series_id !== undefined || seen.has(m.id)) return false;
    seen.add(m.id);
    return true;
  });
}

/** A library series: its two teams, its maps and its known veto. */
export function SeriesRecord({ seriesId }: { seriesId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const series = await getLibrarySeries(seriesId);
        const [matches, candidates] = await Promise.all([listLibraryMatches({ series_id: seriesId }), seriesCandidates(series)]);
        if (active) setState({ kind: 'ready', data: { series, matches, candidates } });
      } catch (error) {
        if (!active) return;
        setState(isLibraryNotFound(error) && !isLibraryOffline(error) ? { kind: 'missing' } : { kind: 'offline' });
      }
    })();
    return () => {
      active = false;
    };
  }, [seriesId, attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando serie" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={Layers}
        title="Serie no encontrada"
        description="Puede que se haya eliminado o que el enlace sea antiguo."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={LIBRARY_SERIES_HREF}>Ver series</Link>
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
        description="No se pudo leer la serie. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <SeriesRecordView data={state.data} onChanged={reload} />;
}

function SeriesRecordView({ data, onChanged }: { data: Loaded; onChanged: () => void }): ReactNode {
  const { series, matches, candidates } = data;
  const router = useRouter();
  const [deleting, setDeleting] = useState(false);
  const name = seriesName(series);
  useRouteTitle(name);
  useDocumentTitle(`${name} · Serie · ClipHub`);

  async function remove(): Promise<void> {
    await deleteLibrarySeries(series.id);
    router.push(LIBRARY_SERIES_HREF);
  }

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Serie"
        icon={Layers}
        title={name}
        subtitle={
          series.teams.length > 0 ? (
            <span className="flex flex-wrap gap-x-3">
              {series.teams.map((team) => (
                <Link key={team.slot} href={teamHref(team.team_id)} className="text-fg-1 hover:text-primary hover:underline">
                  {team.slot.toUpperCase()}: {team.name}
                </Link>
              ))}
            </span>
          ) : (
            'Sin equipos asignados'
          )
        }
        fields={[
          { label: 'Formato', value: SERIES_FORMAT_LABEL[series.format] },
          { label: 'Evento', value: series.event || 'Sin evento' },
          { label: 'Mapas', value: matches.length },
          { label: 'Pasos de veto', value: series.vetoes.length },
          { label: 'Creada', value: formatShortDate(series.created_at) },
        ]}
        actions={
          <>
            <Button type="button" variant="outline" size="sm" onClick={() => setDeleting(true)}>
              <Trash2 aria-hidden />
              Eliminar
            </Button>
            <Button asChild variant="ghost" size="sm">
              <Link href={LIBRARY_SERIES_HREF}>
                <ArrowLeft aria-hidden />
                Todas las series
              </Link>
            </Button>
          </>
        }
      />
      <RecordTabs
        tabs={[
          { id: 'mapas', label: 'Mapas', count: matches.length, content: <SeriesMatches series={series} matches={matches} candidates={candidates} onChanged={onChanged} /> },
          { id: 'veto', label: 'Veto', count: series.vetoes.length, content: <SeriesVeto series={series} matches={matches} onChanged={onChanged} /> },
          { id: 'datos', label: 'Datos', content: <SeriesDetails series={series} onChanged={onChanged} /> },
        ]}
      />
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Eliminar {name}</DialogTitle>
            <DialogDescription>Se borra la serie y su veto. Sus partidas siguen en la biblioteca, sin serie.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleting(false)}>
              Cancelar
            </Button>
            <Button type="button" variant="destructive" onClick={() => void remove()}>
              Eliminar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function SeriesMatches({
  series,
  matches,
  candidates,
  onChanged,
}: {
  series: LibrarySeries;
  matches: readonly LibraryMatch[];
  candidates: readonly LibraryMatch[];
  onChanged: () => void;
}): ReactNode {
  const [adding, setAdding] = useState(NO_MATCH);
  const [error, setError] = useState<string | null>(null);

  async function move(matchId: string, seriesId: string | null): Promise<void> {
    setError(null);
    try {
      await patchLibraryMatch(matchId, { series_id: seriesId });
      setAdding(NO_MATCH);
      onChanged();
    } catch {
      setError('No se pudo cambiar la serie de la partida.');
    }
  }

  const ordered = [...matches].sort((a, b) => a.played_at.localeCompare(b.played_at));
  return (
    <div className="flex flex-col gap-3">
      <RelatedList title="Mapas de la serie" count={matches.length}>
        {ordered.length === 0 ? (
          <p className="px-4 py-3 text-body-sm text-fg-2">Añade las partidas de la serie desde la lista de abajo.</p>
        ) : (
          <ol className="flex flex-col divide-y divide-border-subtle">
            {ordered.map((match, index) => (
              <li key={match.id} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-body-sm">
                <Link href={matchHref(match.id)} className="flex min-w-0 flex-wrap items-baseline gap-x-3 hover:text-primary">
                  <span className="text-fg-3">Mapa {index + 1}</span>
                  <span className="font-medium text-fg-1">{prettyMapName(match.map)}</span>
                  <span className="text-fg-2">{matchTitle(match)}</span>
                  <span className="text-fg-1">{matchScoreLabel(match)}</span>
                  <span className="text-fg-3">{formatShortDate(match.played_at)}</span>
                </Link>
                <Button type="button" size="sm" variant="ghost" onClick={() => void move(match.id, null)}>
                  <X aria-hidden />
                  Quitar
                </Button>
              </li>
            ))}
          </ol>
        )}
      </RelatedList>
      <div className="flex flex-wrap items-center gap-2">
        <Select value={adding} onValueChange={setAdding}>
          <SelectTrigger aria-label="Partida que añadir" className="h-9 min-w-72">
            <SelectValue placeholder={candidates.length === 0 ? 'No hay partidas sin serie de estos equipos' : 'Elige una partida'} />
          </SelectTrigger>
          <SelectContent>
            {candidates.map((match) => (
              <SelectItem key={match.id} value={match.id}>
                {prettyMapName(match.map)} · {matchTitle(match)} · {formatShortDate(match.played_at)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button type="button" size="sm" variant="outline" disabled={adding === NO_MATCH} onClick={() => void move(adding, series.id)}>
          <Plus aria-hidden />
          Añadir a la serie
        </Button>
      </div>
      {error ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}

function SeriesVeto({ series, matches, onChanged }: { series: LibrarySeries; matches: readonly LibraryMatch[]; onChanged: () => void }): ReactNode {
  const [steps, setSteps] = useState<VetoStep[]>(series.vetoes);
  const [notes, setNotes] = useState(series.veto ?? '');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const hasTeams = series.team_ids.length === 2;
  const problem = vetoStepsProblem(steps, hasTeams);
  const mapChoices = [...new Set([...DEFAULT_MAP_POOL, ...EXTRA_POOL_MAPS, ...matches.map((m) => m.map), ...steps.map((s) => s.map)])]
    .filter((map) => map !== '')
    .sort();
  const teamName = (slot: VetoTeam): string => {
    if (slot === '') return 'Ninguno';
    return series.teams.find((team) => team.slot === slot)?.name ?? `Equipo ${slot.toUpperCase()}`;
  };

  function update(index: number, patch: Partial<VetoStep>): void {
    setSteps((current) =>
      current.map((step, i) => {
        if (i !== index) return step;
        const next = { ...step, ...patch };
        if (next.action === 'decider') return { ...next, team: '' };
        return next.team === '' ? { ...next, team: 'a' } : next;
      }),
    );
  }

  async function save(): Promise<void> {
    if (problem !== null) return;
    setBusy(true);
    setMessage(null);
    try {
      await patchLibrarySeries(series.id, { vetoes: steps, veto: notes });
      setMessage('Veto guardado.');
      onChanged();
    } catch {
      setMessage('No se pudo guardar el veto.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      {hasTeams ? null : (
        <p className="text-body-sm text-warning">Asigna los dos equipos en la pestaña Datos para apuntar quién hizo cada pick y ban.</p>
      )}
      <RelatedList
        title="Orden del veto"
        count={steps.length}
        actions={
          <>
            <Button type="button" size="sm" variant="outline" onClick={() => setSteps(seriesVetoTemplate(series.format, DEFAULT_MAP_POOL.length, 'a', steps))}>
              <Wand2 aria-hidden />
              Orden de {SERIES_FORMAT_LABEL[series.format]}
            </Button>
            <Button type="button" size="sm" variant="outline" disabled={steps.length >= 9} onClick={() => setSteps([...steps, { team: 'a', action: 'ban', map: '' }])}>
              <Plus aria-hidden />
              Añadir paso
            </Button>
          </>
        }
      >
        {steps.length === 0 ? (
          <p className="px-4 py-3 text-body-sm text-fg-2">Sin veto apuntado. Si lo conoces, apúntalo: el simulador lo usa para leer las costumbres de cada equipo.</p>
        ) : (
          <ol className="flex flex-col divide-y divide-border-subtle">
            {steps.map((step, index) => (
              <li key={index} className="flex flex-wrap items-center gap-2 px-3 py-2">
                <span className="w-6 text-fg-3">{index + 1}</span>
                <Select value={step.action} onValueChange={(action: VetoAction) => update(index, { action })}>
                  <SelectTrigger aria-label={`Acción del paso ${index + 1}`} className="h-8 w-28">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {VETO_ACTIONS.map((action) => (
                      <SelectItem key={action} value={action}>
                        {VETO_ACTION_LABEL[action]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {step.action === 'decider' ? (
                  <span className="w-44 text-body-sm text-fg-3">Sin equipo</span>
                ) : (
                  <Select value={step.team} onValueChange={(team: VetoTeam) => update(index, { team })}>
                    <SelectTrigger aria-label={`Equipo del paso ${index + 1}`} className="h-8 w-44">
                      <SelectValue>{teamName(step.team)}</SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="a">{teamName('a')}</SelectItem>
                      <SelectItem value="b">{teamName('b')}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
                <Select value={step.map} onValueChange={(map: string) => update(index, { map })}>
                  <SelectTrigger aria-label={`Mapa del paso ${index + 1}`} className="h-8 w-40">
                    <SelectValue placeholder="Mapa" />
                  </SelectTrigger>
                  <SelectContent>
                    {mapChoices.map((map) => (
                      <SelectItem key={map} value={map}>
                        {prettyMapName(map)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button type="button" size="icon-xs" variant="ghost" aria-label={`Quitar el paso ${index + 1}`} onClick={() => setSteps(steps.filter((_, i) => i !== index))}>
                  <X aria-hidden />
                </Button>
              </li>
            ))}
          </ol>
        )}
      </RelatedList>
      <label className="flex flex-col gap-1.5 text-meta text-fg-3">
        Notas del veto
        <textarea
          value={notes}
          maxLength={480}
          onChange={(event) => setNotes(event.target.value)}
          rows={3}
          className="rounded-md border border-border bg-surface-1 px-3 py-2 text-body-sm text-fg-1 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        />
      </label>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" size="sm" disabled={problem !== null} loading={busy} loadingText="Guardando" onClick={() => void save()}>
          Guardar veto
        </Button>
        {problem ? <span className="text-meta text-warning">{problem}</span> : null}
        {message ? (
          <span role="status" className="text-meta text-fg-2">
            {message}
          </span>
        ) : null}
      </div>
    </div>
  );
}

function SeriesDetails({ series, onChanged }: { series: LibrarySeries; onChanged: () => void }): ReactNode {
  const teams = useLibraryTeams();
  const [format, setFormat] = useState<SeriesFormat>(series.format);
  const [event, setEvent] = useState(series.event);
  const [teamA, setTeamA] = useState(series.teams.find((t) => t.slot === 'a')?.team_id ?? NO_TEAM);
  const [teamB, setTeamB] = useState(series.teams.find((t) => t.slot === 'b')?.team_id ?? NO_TEAM);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const halfTeams = (teamA === NO_TEAM) !== (teamB === NO_TEAM);
  const dropsVeto = !halfTeams && teamA === NO_TEAM && series.vetoes.some((step) => step.action !== 'decider');

  async function save(): Promise<void> {
    if (halfTeams || dropsVeto) return;
    setBusy(true);
    setMessage(null);
    try {
      await patchLibrarySeries(series.id, { format, event, team_ids: teamA === NO_TEAM ? [] : [teamA, teamB] });
      setMessage('Datos guardados.');
      onChanged();
    } catch {
      setMessage('No se pudieron guardar los datos.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex max-w-xl flex-col gap-4">
      <Segmented
        label="Formato"
        value={format}
        options={SERIES_FORMATS.map((value) => ({ value, label: SERIES_FORMAT_LABEL[value] }))}
        onChange={setFormat}
      />
      <label className="flex flex-col gap-1.5 text-meta text-fg-3">
        Evento
        <Input value={event} maxLength={120} onChange={(e) => setEvent(e.target.value)} />
      </label>
      {teams === null ? (
        <Skeleton className="h-9 w-full" />
      ) : (
        <div className="flex flex-wrap gap-2">
          <SeriesTeamSelect label="Equipo A" value={teamA} teams={teams} exclude={teamB} onChange={setTeamA} />
          <SeriesTeamSelect label="Equipo B" value={teamB} teams={teams} exclude={teamA} onChange={setTeamB} />
        </div>
      )}
      {halfTeams ? <p className="text-meta text-warning">Elige los dos equipos o ninguno.</p> : null}
      {dropsVeto ? <p className="text-meta text-warning">El veto apuntado necesita los dos equipos.</p> : null}
      <div className="flex items-center gap-3">
        <Button type="button" size="sm" disabled={halfTeams || dropsVeto} loading={busy} loadingText="Guardando" onClick={() => void save()}>
          Guardar datos
        </Button>
        {message ? (
          <span role="status" className="text-meta text-fg-2">
            {message}
          </span>
        ) : null}
      </div>
    </div>
  );
}
