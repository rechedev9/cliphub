'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { BookmarkPlus } from 'lucide-react';
import {
  createLibraryPlay,
  detectTeamPlays,
  listLibraryMatches,
  listLibraryTeams,
  type LibraryMatch,
  type LibraryTeam,
} from '@/lib/api/library';
import type { ReportSide, ScoutEvidence } from '@/lib/api/scout';
import type { DetectedPlay, LibraryPlay, PlayDetection as Detection } from '@/lib/api/stratbook';
import { playHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { rivalOf, teamLabel } from '@/lib/library/partidas';
import { evidencePath, rateText } from '@/lib/reports/scout-sections';
import { playFromDetected, playShapeLabel } from '@/lib/stratbook/plays';
import { ReportSideSelect } from '@/components/reports/new-report-dialog';
import { ANY, OptionSelect } from '@/components/teams/team-scout';
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
import { PlayGrenadeTable, PlayPositionTable } from './play-tables';

type Filters = { map: string; side: ReportSide };

/** The rounds a team repeats, grouped by the orchestrator, with a door into the stratbook. */
export function PlayDetection({ initialTeamId, onSaved }: { initialTeamId?: string; onSaved: () => void }): ReactNode {
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  const [teamId, setTeamId] = useState(initialTeamId ?? '');
  const [matches, setMatches] = useState<LibraryMatch[]>([]);
  const [filters, setFilters] = useState<Filters>({ map: ANY, side: '' });
  const [result, setResult] = useState<Detection | null | 'failed'>(null);
  const [saving, setSaving] = useState<DetectedPlay | null>(null);

  useEffect(() => {
    let active = true;
    listLibraryTeams()
      .then((list) => {
        if (active) setTeams(list.filter((team) => team.match_count > 0).sort((a, b) => a.name.localeCompare(b.name, 'es')));
      })
      .catch(() => {
        if (active) setTeams([]);
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (teamId === '') return;
    let active = true;
    setMatches([]);
    listLibraryMatches({ team_id: teamId })
      .then((list) => {
        if (active) setMatches(list);
      })
      .catch(() => {
        if (active) setMatches([]);
      });
    return () => {
      active = false;
    };
  }, [teamId]);

  useEffect(() => {
    if (teamId === '') return;
    let active = true;
    setResult(null);
    detectTeamPlays(teamId, { maps: filters.map === ANY ? [] : [filters.map], side: filters.side })
      .then((next) => {
        if (active) setResult(next);
      })
      .catch(() => {
        if (active) setResult('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, filters]);

  const maps = [...new Set(matches.map((match) => match.map))].sort().map((map) => ({ value: map, label: prettyMapName(map) }));
  const labels = new Map(
    matches.map((match) => {
      const rival = rivalOf(match, teamId);
      return [match.id, `${prettyMapName(match.map)}${rival ? ` vs ${teamLabel(rival)}` : ''}`] as const;
    }),
  );

  return (
    <div className="flex flex-col gap-4">
      <p className="measure-read text-body-sm text-fg-2">
        Studio agrupa las rondas de un equipo que comparten las granadas (tipo y zona donde caen) y las posiciones de sus jugadores, en el mismo mapa y lado. Las reglas están debajo de los resultados.
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <Select value={teamId} onValueChange={(next) => setTeamId(next)}>
          <SelectTrigger aria-label="Equipo" className="h-9 min-w-48">
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
        <OptionSelect label="Mapa" value={filters.map} options={maps} onChange={(map) => setFilters({ ...filters, map })} />
        <div className="w-36">
          <ReportSideSelect value={filters.side} onChange={(side) => setFilters({ ...filters, side })} />
        </div>
      </div>
      {teamId === '' ? (
        <p className="text-body-sm text-fg-2">{teams?.length === 0 ? 'Aún no hay equipos con partidas analizadas.' : 'Elige un equipo para buscar sus jugadas repetidas.'}</p>
      ) : (
        <DetectionResult result={result} labels={labels} onSave={setSaving} />
      )}
      {saving === null ? null : (
        <SaveDetectedDialog
          play={saving}
          teamId={teamId}
          teamName={teams?.find((team) => team.id === teamId)?.name ?? ''}
          positionSeconds={result !== null && result !== 'failed' ? result.thresholds.position_seconds : 0}
          onClose={() => setSaving(null)}
          onSaved={onSaved}
        />
      )}
    </div>
  );
}

function DetectionResult({
  result,
  labels,
  onSave,
}: {
  result: Detection | null | 'failed';
  labels: ReadonlyMap<string, string>;
  onSave: (play: DetectedPlay) => void;
}): ReactNode {
  if (result === 'failed') {
    return <p role="alert" className="text-body-sm text-destructive">No se pudieron leer las partidas de este equipo. Comprueba que ClipHub Studio sigue abierto.</p>;
  }
  if (result === null) {
    return (
      <div role="status" aria-label="Buscando jugadas repetidas" className="flex flex-col gap-3">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }
  const t = result.thresholds;
  let headline = `${result.plays.length} jugadas repetidas.`;
  if (result.plays.length === 0) headline = 'No hay jugadas repetidas con estas reglas.';
  if (result.plays.length === 1) headline = '1 jugada repetida.';
  return (
    <div className="flex flex-col gap-4">
      <p role="status" className="text-body-sm text-fg-1">
        {headline}{' '}
        <span className="text-fg-2">
          {result.rounds} rondas leídas en {result.matches} {result.matches === 1 ? 'partida' : 'partidas'}; {result.compared} con utilidad suficiente para compararse.
        </span>
      </p>
      {result.plays.map((play) => (
        <DetectedPlayCard key={`${play.seed.match_id}:${play.seed.round}`} play={play} labels={labels} onSave={() => onSave(play)} />
      ))}
      <p className="measure-read text-meta text-fg-3">
        Reglas: una ronda se compara si su equipo no va de eco y lanza al menos {t.min_grenades} granadas antes de la plantada. Dos rondas son la misma
        jugada si comparten al menos el {Math.round(t.min_utility_similarity * 100)} % de sus granadas (tipo y zona) y el{' '}
        {Math.round(t.min_position_similarity * 100)} % de las zonas que ocupan a los {t.position_seconds} s; sin posiciones solo cuentan las granadas.
        Una jugada necesita {t.min_rounds} rondas, y lista las granadas y zonas que aparecen en al menos el {Math.round(t.common_share * 100)} % de ellas.
      </p>
    </div>
  );
}

function EvidenceLinks({ examples, labels }: { examples: readonly ScoutEvidence[]; labels: ReadonlyMap<string, string> }): ReactNode {
  const many = new Set(examples.map((example) => example.match_id)).size > 1;
  return (
    <span className="flex flex-wrap gap-1">
      {examples.map((example) => (
        <Link
          key={`${example.match_id}:${example.round}`}
          href={evidencePath(example)}
          title={`${labels.get(example.match_id) ?? 'Partida'}, ronda ${example.round}, tick ${example.tick}`}
          className="rounded-sm border border-border px-1.5 text-meta whitespace-nowrap text-fg-2 hover:border-primary hover:text-primary"
        >
          {many ? `${labels.get(example.match_id) ?? 'Partida'} R${example.round}` : `R${example.round}`}
        </Link>
      ))}
    </span>
  );
}

function DetectedPlayCard({ play, labels, onSave }: { play: DetectedPlay; labels: ReadonlyMap<string, string>; onSave: () => void }): ReactNode {
  return (
    <section className="studio-panel flex flex-col gap-3 px-4 py-3" aria-label={`${playShapeLabel(play)} en ${prettyMapName(play.map)}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-0.5">
          <h3 className="text-body font-semibold text-fg-1">
            {playShapeLabel(play)} <span className="text-fg-2">· {play.side} · {prettyMapName(play.map)}</span>
          </h3>
          <p className="text-body-sm text-fg-2 tabular-nums">
            {play.rounds} rondas ({rateText(play.share)} de sus rondas en este mapa y lado) · gana {rateText(play.win_rate)}
          </p>
        </div>
        <Button type="button" size="sm" onClick={onSave}>
          <BookmarkPlus aria-hidden />
          Guardar en el stratbook
        </Button>
      </div>
      <div className="grid gap-3 lg:grid-cols-2">
        <PlayGrenadeTable grenades={play.grenades} showShare />
        <PlayPositionTable positions={play.positions} showShare />
      </div>
      <div className="flex flex-col gap-1">
        <span className="text-meta text-fg-3">Rondas de la jugada (la primera es la de referencia)</span>
        <EvidenceLinks examples={play.examples} labels={labels} />
      </div>
    </section>
  );
}

function SaveDetectedDialog({
  play,
  teamId,
  teamName,
  positionSeconds,
  onClose,
  onSaved,
}: {
  play: DetectedPlay;
  teamId: string;
  teamName: string;
  positionSeconds: number;
  onClose: () => void;
  onSaved: () => void;
}): ReactNode {
  const [title, setTitle] = useState(`${playShapeLabel(play)} de ${teamName}`.trim());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<LibraryPlay | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      setSaved(await createLibraryPlay(playFromDetected(play, { teamId, title, positionSeconds })));
      onSaved();
    } catch {
      setError('No se pudo guardar la jugada.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        {saved ? (
          <>
            <DialogHeader>
              <DialogTitle>Jugada guardada</DialogTitle>
              <DialogDescription>{saved.title} ya está en el stratbook con sus {saved.examples.length} rondas de ejemplo.</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                Seguir aquí
              </Button>
              <Button asChild>
                <Link href={playHref(saved.id)}>Abrir la jugada</Link>
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>Guardar la jugada repetida</DialogTitle>
              <DialogDescription>Se guardan sus granadas y posiciones comunes, y sus {play.rounds} rondas como ejemplos.</DialogDescription>
            </DialogHeader>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Título
              <Input value={title} maxLength={120} onChange={(event) => setTitle(event.target.value)} />
            </label>
            {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
                Cancelar
              </Button>
              <Button type="submit" disabled={busy || title.trim() === ''}>
                Guardar jugada
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
