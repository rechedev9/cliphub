'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { BookmarkPlus, Clapperboard, Crosshair } from 'lucide-react';
import { LIBRARY_TEAM_KEYS, createLibraryPlay, getRoundLineups, getRoundPlayDraft, type LibraryTeamKey } from '@/lib/api/library';
import type { LibraryPlay, RoundLineups } from '@/lib/api/stratbook';
import type { TacticalDocument, TacticalRound, TacticalSide } from '@/lib/api/tactical';
import { matchHref, playHref } from '@/lib/clips/routes';
import { roundEvidenceHref, teamKeyOnSide } from '@/lib/library/rounds';
import { LINEUP_STATUS_LABEL, playFromDraft, playSecondsLabel, playShapeLabel, suggestPlayTitle } from '@/lib/stratbook/plays';
import { eventKindLabel } from '@/lib/tactical-labels';
import { Segmented } from '@/components/tactical/spatial/spatial-controls';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
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
import { Skeleton } from '@/components/ui/skeleton';
import { PlayGrenadeTable, PlayPositionTable } from './play-tables';

function isTeamKey(key: string): key is LibraryTeamKey {
  return LIBRARY_TEAM_KEYS.some((entry) => entry === key);
}

/**
 * The stratbook and video doors of a round replay: make a video of the round,
 * save it as a play (with the positions at the tick the coach is on) and
 * compare its grenades with the lineup catalog. Only a library match has them.
 */
export function RoundPlayActions({
  matchId,
  doc,
  round,
  currentTick,
}: {
  matchId: string;
  doc: TacticalDocument;
  round: TacticalRound;
  /** Reads the replay's tick when a dialog opens, so playback never re-renders this. */
  currentTick: () => number;
}): ReactNode {
  const [saving, setSaving] = useState<{ tick: number } | null>(null);
  const [comparing, setComparing] = useState(false);
  const [video, setVideo] = useState(false);
  return (
    <div className="flex flex-wrap gap-2">
      <Button type="button" size="sm" variant="outline" onClick={() => setVideo(true)}>
        <Clapperboard aria-hidden />
        Crear vídeo
      </Button>
      <Button type="button" size="sm" variant="outline" onClick={() => setSaving({ tick: currentTick() })}>
        <BookmarkPlus aria-hidden />
        Guardar como jugada
      </Button>
      <Button type="button" size="sm" variant="ghost" onClick={() => setComparing(true)}>
        <Crosshair aria-hidden />
        Lineups
      </Button>
      {saving === null ? null : (
        <SavePlayDialog matchId={matchId} doc={doc} round={round} tick={saving.tick} onClose={() => setSaving(null)} />
      )}
      {comparing ? <RoundLineupsDialog matchId={matchId} round={round.number} onClose={() => setComparing(false)} /> : null}
      {video ? (
        <AnalysisVideoDialog
          title={`Crear vídeo de la ronda ${round.number}`}
          sources={[{ matchId, rounds: [round.number] }]}
          onClose={() => setVideo(false)}
        />
      ) : null}
    </div>
  );
}

function SavePlayDialog({
  matchId,
  doc,
  round,
  tick,
  onClose,
}: {
  matchId: string;
  doc: TacticalDocument;
  round: TacticalRound;
  tick: number;
  onClose: () => void;
}): ReactNode {
  const [side, setSide] = useState<TacticalSide>('T');
  const [draft, setDraft] = useState<LibraryPlay | null | 'failed'>(null);
  const [title, setTitle] = useState('');
  const [titleEdited, setTitleEdited] = useState(false);
  const [notes, setNotes] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<LibraryPlay | null>(null);
  const teamKey = teamKeyOnSide(doc, round, side);
  const teamName = (which: TacticalSide): string => doc.teams.find((team) => team.key === teamKeyOnSide(doc, round, which))?.name ?? which;

  useEffect(() => {
    if (!isTeamKey(teamKey)) {
      setDraft('failed');
      return;
    }
    let active = true;
    setDraft(null);
    getRoundPlayDraft(matchId, round.number, teamKey, tick)
      .then((next) => {
        if (active) setDraft(next);
      })
      .catch(() => {
        if (active) setDraft('failed');
      });
    return () => {
      active = false;
    };
  }, [matchId, round.number, teamKey, tick]);

  useEffect(() => {
    if (!titleEdited && draft !== null && draft !== 'failed') setTitle(suggestPlayTitle(draft, round.number));
  }, [draft, round.number, titleEdited]);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (draft === null || draft === 'failed') return;
    setBusy(true);
    setError(null);
    try {
      setSaved(await createLibraryPlay(playFromDraft(draft, title, notes)));
    } catch {
      setError('No se pudo guardar la jugada. Comprueba que la partida sigue en la biblioteca.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-2xl" showCloseButton={!busy}>
        {saved ? (
          <>
            <DialogHeader>
              <DialogTitle>Jugada guardada</DialogTitle>
              <DialogDescription>{saved.title} ya está en el stratbook, con esta ronda como ejemplo.</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                Seguir en la repetición
              </Button>
              <Button asChild>
                <Link href={playHref(saved.id)}>Abrir la jugada</Link>
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>Guardar la ronda {round.number} como jugada</DialogTitle>
              <DialogDescription>
                Se guardan las granadas del equipo hasta la plantada y dónde estaba cada jugador en el momento de la repetición en que estás.
              </DialogDescription>
            </DialogHeader>
            <Segmented
              label="Equipo"
              value={side}
              options={(['T', 'CT'] as const).map((value) => ({ value, label: `${value}: ${teamName(value)}` }))}
              onChange={(next) => setSide(next)}
            />
            {draft === null ? <Skeleton className="h-40 w-full" /> : null}
            {draft === 'failed' ? (
              <p role="alert" className="text-body-sm text-destructive">No se pudo leer la ronda para este equipo.</p>
            ) : null}
            {draft !== null && draft !== 'failed' ? (
              <>
                <p className="text-body-sm text-fg-2">
                  {playShapeLabel(draft)} en {draft.side}
                  {draft.team_name ? ` de ${draft.team_name}` : ''}. Posiciones a {playSecondsLabel(draft.positions_seconds)} del final del freeze time.
                </p>
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
                  Notas
                  <textarea
                    value={notes}
                    maxLength={4000}
                    rows={3}
                    onChange={(event) => setNotes(event.target.value)}
                    className="rounded-md border border-input bg-transparent px-3 py-2 text-body-sm text-fg-1 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  />
                </label>
                <div className="flex max-h-[36vh] flex-col gap-3 overflow-y-auto">
                  <PlayGrenadeTable grenades={draft.grenades} />
                  <PlayPositionTable positions={draft.positions} />
                </div>
              </>
            ) : null}
            {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
                Cancelar
              </Button>
              <Button type="submit" disabled={busy || draft === null || draft === 'failed' || title.trim() === ''}>
                Guardar jugada
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

function RoundLineupsDialog({ matchId, round, onClose }: { matchId: string; round: number; onClose: () => void }): ReactNode {
  const [result, setResult] = useState<RoundLineups | null | 'failed'>(null);

  useEffect(() => {
    let active = true;
    getRoundLineups(matchId, round)
      .then((next) => {
        if (active) setResult(next);
      })
      .catch(() => {
        if (active) setResult('failed');
      });
    return () => {
      active = false;
    };
  }, [matchId, round]);

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Granadas de la ronda {round} y el catálogo de lineups</DialogTitle>
          <DialogDescription>
            Cada humo se compara por el punto donde cayó con los lineups del catálogo de Studio. La demo no guarda desde dónde se lanzó, así que la posición de lanzamiento no se comprueba.
          </DialogDescription>
        </DialogHeader>
        {result === null ? <Skeleton className="h-40 w-full" /> : null}
        {result === 'failed' ? (
          <p role="alert" className="text-body-sm text-destructive">No se pudo leer la ronda. Comprueba que el análisis de la partida está listo.</p>
        ) : null}
        {result !== null && result !== 'failed' ? <RoundLineupsTable matchId={matchId} result={result} /> : null}
      </DialogContent>
    </Dialog>
  );
}

const CELL = 'px-2 py-1.5 text-left font-normal';

function RoundLineupsTable({ matchId, result }: { matchId: string; result: RoundLineups }): ReactNode {
  const summary =
    result.catalog_size === 0
      ? 'El catálogo no tiene lineups de este mapa todavía.'
      : `${result.matched} de ${result.smokes} humos coinciden con un lineup del catálogo (${result.catalog_size} lineups de este mapa).`;
  return (
    <div className="flex flex-col gap-3">
      <p role="status" className="text-body-sm text-fg-1">{summary}</p>
      {result.grenades.length === 0 ? (
        <p className="text-body-sm text-fg-2">La ronda no tiene granadas.</p>
      ) : (
        <div className="max-h-[50vh] overflow-auto rounded-md border border-border">
          <table className="w-full min-w-[40rem] text-body-sm tabular-nums">
            <caption className="sr-only">Granadas de la ronda frente al catálogo</caption>
            <thead className="bg-surface-1 text-meta text-fg-3">
              <tr>
                <th scope="col" className={CELL}>Momento</th>
                <th scope="col" className={CELL}>Granada</th>
                <th scope="col" className={CELL}>Jugador</th>
                <th scope="col" className={CELL}>Zona</th>
                <th scope="col" className={CELL}>Catálogo</th>
              </tr>
            </thead>
            <tbody>
              {result.grenades.map((grenade) => (
                <tr key={`${grenade.tick}:${grenade.kind}:${grenade.pos.join(',')}`} className="border-t border-border-subtle">
                  <td className={CELL}>
                    <Link
                      href={roundEvidenceHref(matchHref(matchId), result.round, grenade.tick)}
                      className="font-mono text-fg-2 hover:text-primary hover:underline"
                    >
                      {playSecondsLabel(grenade.seconds)}
                    </Link>
                  </td>
                  <td className={CELL}>
                    {eventKindLabel(grenade.kind)}
                    {grenade.side ? <span className="pl-1 text-fg-3">{grenade.side}</span> : null}
                  </td>
                  <td className={CELL}>{grenade.player || <span className="text-fg-3">Sin dato</span>}</td>
                  <td className={CELL}>{grenade.callout || <span className="text-fg-3">Sin zona</span>}</td>
                  <td className={CELL}>
                    {grenade.status === 'matched' && grenade.lineup ? (
                      <span className="text-fg-1">
                        {grenade.lineup.destination}
                        {grenade.lineup.from_area ? ` desde ${grenade.lineup.from_area}` : ''}
                        <span className="pl-1 text-fg-3">{Math.round((grenade.lineup.confidence ?? 0) * 100)} %</span>
                      </span>
                    ) : (
                      <span className="text-fg-3">{LINEUP_STATUS_LABEL[grenade.status]}</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
