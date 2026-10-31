'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowLeft, BookMarked, Clapperboard, FileDown, Pencil, PlugZap, Trash2 } from 'lucide-react';
import {
  deleteLibraryPlay,
  getLibraryPlay,
  isLibraryNotFound,
  listLibraryMatches,
  listLibraryTeams,
  patchLibraryPlay,
  type LibraryMatch,
  type LibraryTeam,
} from '@/lib/api/library';
import { STRATBOOK_FORMAT, STRATBOOK_VERSION, type LibraryPlay, type LibraryPlayDetail } from '@/lib/api/stratbook';
import { TACTICAL_SITE_ORDER, type TacticalSite } from '@/lib/api/tactical';
import { STRATBOOK_HREF, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { rivalOf, teamLabel } from '@/lib/library/partidas';
import { groupEvidenceByMatch, type VideoSource } from '@/lib/library/analysis-video';
import { dateText, evidencePath } from '@/lib/reports/scout-sections';
import { PLAY_ORIGIN_LABEL, playKindLabel, playKinds, playSecondsLabel, playShapeLabel, stratbookFileName } from '@/lib/stratbook/plays';
import { siteLabel } from '@/lib/tactical-labels';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
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
import { PlayRadar } from './play-radar';
import { PlayGrenadeTable, PlayPositionTable } from './play-tables';

type LoadState = { kind: 'loading' } | { kind: 'ready'; play: LibraryPlayDetail } | { kind: 'missing' } | { kind: 'offline' };

const NO_TEAM = '__none__';

/** One saved play: what it is, its grenades and positions on the radar, and the rounds it comes from. */
export function PlayRecord({ playId }: { playId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    getLibraryPlay(playId)
      .then((play) => {
        if (active) setState({ kind: 'ready', play });
      })
      .catch((error: unknown) => {
        if (!active) return;
        setState({ kind: isLibraryNotFound(error) ? 'missing' : 'offline' });
      });
    return () => {
      active = false;
    };
  }, [playId, attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando jugada" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={BookMarked}
        title="Jugada no encontrada"
        description="Puede que se haya eliminado o que el enlace sea antiguo."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={STRATBOOK_HREF}>Ver el stratbook</Link>
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
        description="No se pudo leer la jugada. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <PlayRecordView play={state.play} onChanged={reload} />;
}

function downloadJson(value: unknown, fileName: string): void {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: 'application/json;charset=utf-8' }));
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = fileName;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function PlayRecordView({ play, onChanged }: { play: LibraryPlayDetail; onChanged: () => void }): ReactNode {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  /** The example rounds the video dialog offers; null while it is closed. */
  const [video, setVideo] = useState<{ sources: VideoSource[]; selected?: number[] } | null>(null);
  const [matches, setMatches] = useState<ReadonlyMap<string, LibraryMatch>>(new Map());
  const [message, setMessage] = useState<string | null>(null);
  useRouteTitle(play.title);
  useDocumentTitle(`${play.title} · Stratbook · ClipHub`);

  useEffect(() => {
    let active = true;
    listLibraryMatches({ map: play.map })
      .then((list) => {
        if (active) setMatches(new Map(list.map((match) => [match.id, match])));
      })
      .catch(() => {
        if (active) setMatches(new Map());
      });
    return () => {
      active = false;
    };
  }, [play.map]);

  const missing = new Set(play.missing_match_ids);
  const available = play.examples.filter((example) => !missing.has(example.match_id));

  function exportPlay(): void {
    const stored: LibraryPlay = {
      id: play.id, title: play.title, team_id: play.team_id, team_name: play.team_name, map: play.map, side: play.side,
      kind: play.kind, site: play.site, origin: play.origin, positions_seconds: play.positions_seconds, grenades: play.grenades,
      positions: play.positions, notes: play.notes, examples: play.examples, created_at: play.created_at, updated_at: play.updated_at,
    };
    downloadJson(
      { format: STRATBOOK_FORMAT, version: STRATBOOK_VERSION, exported_at: new Date().toISOString(), plays: [stored] },
      stratbookFileName(new Date(), play.title),
    );
    setMessage('Jugada exportada como fichero de stratbook.');
  }

  async function removeExample(index: number): Promise<void> {
    try {
      await patchLibraryPlay(play.id, { examples: play.examples.filter((_, i) => i !== index) });
      onChanged();
    } catch {
      setMessage('No se pudo quitar la ronda.');
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Jugada"
        icon={BookMarked}
        title={play.title}
        subtitle={
          play.team_id ? (
            <>
              Equipo:{' '}
              <Link href={teamHref(play.team_id)} className="text-fg-1 hover:text-primary hover:underline">
                {play.team_name}
              </Link>
            </>
          ) : (
            'Sin equipo'
          )
        }
        fields={[
          { label: 'Mapa', value: prettyMapName(play.map) },
          { label: 'Lado', value: play.side },
          { label: 'Tipo', value: playShapeLabel(play) },
          { label: 'Granadas', value: play.grenades.length },
          { label: 'Rondas de ejemplo', value: play.examples.length },
          { label: 'Origen', value: PLAY_ORIGIN_LABEL[play.origin] },
          { label: 'Actualizada', value: formatShortDate(play.updated_at) },
        ]}
        actions={
          <>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={available.length === 0}
              onClick={() => setVideo({ sources: groupEvidenceByMatch(available) })}
            >
              <Clapperboard aria-hidden />
              Crear vídeo
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setEditing(true)}>
              <Pencil aria-hidden />
              Editar
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={exportPlay}>
              <FileDown aria-hidden />
              Exportar
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => setDeleting(true)}>
              <Trash2 aria-hidden />
              Eliminar
            </Button>
            <Button asChild variant="ghost" size="sm">
              <Link href={STRATBOOK_HREF}>
                <ArrowLeft aria-hidden />
                Todo el stratbook
              </Link>
            </Button>
          </>
        }
      />
      {message ? (
        <p role="status" className="text-body-sm text-fg-2">
          {message}
        </p>
      ) : null}
      {missing.size > 0 ? (
        <p className="rounded-md border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm leading-5 text-warning">
          {missing.size === 1
            ? 'Una partida de ejemplo no está en esta biblioteca (por ejemplo, porque la jugada se importó de otro PC).'
            : `${missing.size} partidas de ejemplo no están en esta biblioteca (por ejemplo, porque la jugada se importó de otro PC).`}{' '}
          Importa sus demos para abrir esas rondas.
        </p>
      ) : null}
      <RecordTabs
        tabs={[
          {
            id: 'resumen',
            label: 'Resumen',
            content: (
              <div className="flex flex-col gap-5">
                <PlayNotes key={play.updated_at} play={play} onSaved={onChanged} />
                <section className="flex flex-col gap-2">
                  <h2 className="text-body font-semibold text-fg-1">Granadas</h2>
                  <PlayGrenadeTable grenades={play.grenades} showShare={play.origin === 'detected'} />
                </section>
                <section className="flex flex-col gap-2">
                  <h2 className="text-body font-semibold text-fg-1">
                    Posiciones{play.positions.length > 0 ? ` a ${playSecondsLabel(play.positions_seconds)} del final del freeze time` : ''}
                  </h2>
                  <PlayPositionTable positions={play.positions} showShare={play.origin === 'detected'} />
                </section>
              </div>
            ),
          },
          {
            id: 'radar',
            label: 'Radar 2D',
            content: <PlayRadar matchId={available[0]?.match_id} side={play.side} grenades={play.grenades} positions={play.positions} />,
          },
          {
            id: 'rondas',
            label: 'Rondas de ejemplo',
            count: play.examples.length,
            content: (
              <ExampleList
                play={play}
                matches={matches}
                missing={missing}
                onRemove={(index) => void removeExample(index)}
                onVideo={(example) => setVideo({ sources: groupEvidenceByMatch([example]), selected: [example.round] })}
              />
            ),
          },
        ]}
      />
      {editing ? <EditPlayDialog play={play} onClose={() => setEditing(false)} onSaved={onChanged} /> : null}
      {video ? (
        <AnalysisVideoDialog
          title={`Crear vídeo de ${play.title}`}
          sources={video.sources}
          selected={video.selected}
          teamId={play.team_id ?? undefined}
          onClose={() => setVideo(null)}
        />
      ) : null}
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Eliminar {play.title}</DialogTitle>
            <DialogDescription>Se borra la jugada del stratbook. Las partidas y sus rondas siguen en la biblioteca.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleting(false)}>
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => {
                deleteLibraryPlay(play.id)
                  .then(() => router.push(STRATBOOK_HREF))
                  .catch(() => {
                    setDeleting(false);
                    setMessage('No se pudo eliminar la jugada.');
                  });
              }}
            >
              Eliminar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function PlayNotes({ play, onSaved }: { play: LibraryPlayDetail; onSaved: () => void }): ReactNode {
  const [notes, setNotes] = useState(play.notes);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await patchLibraryPlay(play.id, { notes });
      onSaved();
    } catch {
      setError('No se pudieron guardar las notas.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={(event) => void save(event)} className="flex flex-col gap-2">
      <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
        Notas
        <textarea
          value={notes}
          maxLength={4000}
          rows={4}
          onChange={(event) => setNotes(event.target.value)}
          className="rounded-md border border-input bg-transparent px-3 py-2 text-body-sm text-fg-1 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
        />
      </label>
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      <div>
        <Button type="submit" size="sm" disabled={busy || notes.trim() === play.notes.trim()}>
          Guardar notas
        </Button>
      </div>
    </form>
  );
}

function ExampleList({
  play,
  matches,
  missing,
  onRemove,
  onVideo,
}: {
  play: LibraryPlayDetail;
  matches: ReadonlyMap<string, LibraryMatch>;
  missing: ReadonlySet<string>;
  onRemove: (index: number) => void;
  onVideo: (example: LibraryPlayDetail['examples'][number]) => void;
}): ReactNode {
  if (play.examples.length === 0) return <p className="text-body-sm text-fg-2">Esta jugada no tiene rondas de ejemplo.</p>;
  return (
    <ul className="flex flex-col divide-y divide-border-subtle rounded-md border border-border">
      {play.examples.map((example, index) => {
        const match = matches.get(example.match_id);
        const rival = match && play.team_id ? rivalOf(match, play.team_id) : undefined;
        let where = missing.has(example.match_id) ? 'Partida que no está en esta biblioteca' : 'Partida';
        if (match) where = `${prettyMapName(match.map)}, ${dateText(match.played_at)}${rival ? `, contra ${teamLabel(rival)}` : ''}`;
        return (
          <li key={`${example.match_id}:${example.round}:${index}`} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
            <span className="text-body-sm text-fg-1">
              Ronda {example.round} <span className="text-fg-2">· {where}</span>
            </span>
            <span className="flex gap-2">
              {missing.has(example.match_id) ? null : (
                <>
                  <Button asChild variant="outline" size="sm">
                    <Link href={evidencePath(example)}>Abrir en la repetición</Link>
                  </Button>
                  <Button type="button" variant="outline" size="sm" onClick={() => onVideo(example)}>
                    <Clapperboard aria-hidden />
                    Vídeo
                  </Button>
                </>
              )}
              <Button type="button" variant="ghost" size="sm" onClick={() => onRemove(index)}>
                Quitar
              </Button>
            </span>
          </li>
        );
      })}
    </ul>
  );
}

function EditPlayDialog({ play, onClose, onSaved }: { play: LibraryPlayDetail; onClose: () => void; onSaved: () => void }): ReactNode {
  const [title, setTitle] = useState(play.title);
  const [kind, setKind] = useState(play.kind);
  const [site, setSite] = useState<TacticalSite>(play.site);
  const [teamId, setTeamId] = useState(play.team_id ?? NO_TEAM);
  const [teams, setTeams] = useState<LibraryTeam[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

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

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await patchLibraryPlay(play.id, { title: title.trim(), kind, site, team_id: teamId === NO_TEAM ? null : teamId });
      onSaved();
      onClose();
    } catch {
      setError('No se pudo guardar la jugada.');
    } finally {
      setBusy(false);
    }
  }

  const isSite = (value: string): value is TacticalSite => TACTICAL_SITE_ORDER.some((entry) => entry === value);

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Editar la jugada</DialogTitle>
            <DialogDescription>El mapa y el lado son los de las rondas de las que sale, así que no cambian.</DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Título
            <Input value={title} maxLength={120} onChange={(event) => setTitle(event.target.value)} />
          </label>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Tipo
              <Select value={kind} onValueChange={setKind}>
                <SelectTrigger aria-label="Tipo" className="h-9 w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {playKinds(play.side).map((value) => (
                    <SelectItem key={value} value={value}>
                      {playKindLabel(value)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Sitio
              <Select value={site} onValueChange={(value) => isSite(value) && setSite(value)}>
                <SelectTrigger aria-label="Sitio" className="h-9 w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TACTICAL_SITE_ORDER.map((value) => (
                    <SelectItem key={value} value={value}>
                      {siteLabel(value)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
          </div>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Equipo
            <Select value={teamId} onValueChange={setTeamId}>
              <SelectTrigger aria-label="Equipo" className="h-9 w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NO_TEAM}>Sin equipo</SelectItem>
                {teams.map((team) => (
                  <SelectItem key={team.id} value={team.id}>
                    {team.name}
                  </SelectItem>
                ))}
                {play.team_id && !teams.some((team) => team.id === play.team_id) ? (
                  <SelectItem value={play.team_id}>{play.team_name ?? 'Equipo actual'}</SelectItem>
                ) : null}
              </SelectContent>
            </Select>
          </label>
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || title.trim() === ''}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
