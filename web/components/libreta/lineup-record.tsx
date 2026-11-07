'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Copy, Crosshair, Pencil, Play, PlugZap, Trash2 } from 'lucide-react';
import { isLibraryNotFound } from '@/lib/api/library';
import { deleteLibraryLineup, getLibraryLineup, patchLibraryLineup, type LibraryLineup } from '@/lib/api/lineups';
import { LIBRETA_TABS, libretaTabHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { lineupSpot, setposCommand } from '@/lib/libreta/lineups';
import { evidencePath } from '@/lib/reports/scout-sections';
import { EmptyCell } from '@/components/records/empty-cell';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { PracticeResult, usePracticeLaunch } from '@/components/patterns/practice-launch';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { useCalloutNames } from '@/hooks/use-callout-names';
import { EditLineupDialog } from './edit-lineup-dialog';
import { GrenadeName, TechniqueText } from './lineup-fields';
import { LineupRadar } from './lineup-radar';
import { StatusBadge } from './status-badge';

const LINEUPS_HREF = libretaTabHref(LIBRETA_TABS.lineups);

type LoadState = { kind: 'loading' } | { kind: 'ready'; lineup: LibraryLineup } | { kind: 'missing' } | { kind: 'offline' };

/** One saved lineup: where to stand and aim, how to throw it, and a launch into CS2 to practise it. */
export function LineupRecord({ lineupId }: { lineupId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    getLibraryLineup(lineupId)
      .then((lineup) => {
        if (active) setState({ kind: 'ready', lineup });
      })
      .catch((error: unknown) => {
        if (active) setState({ kind: isLibraryNotFound(error) ? 'missing' : 'offline' });
      });
    return () => {
      active = false;
    };
  }, [lineupId, attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando lineup" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={Crosshair}
        title="Lineup no encontrado"
        description="Puede que se haya eliminado o que el enlace sea antiguo."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={LINEUPS_HREF}>Ver los lineups</Link>
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
        description="No se pudo leer el lineup. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <LineupRecordView lineup={state.lineup} onChanged={reload} />;
}

function LineupRecordView({ lineup, onChanged }: { lineup: LibraryLineup; onChanged: () => void }): ReactNode {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const practice = usePracticeLaunch(lineup.map);
  const calloutLabel = useCalloutNames([lineup.map]);
  const command = setposCommand(lineup);
  useRouteTitle(lineup.title);
  useDocumentTitle(`${lineup.title} · Lineup · ClipHub`);

  function copySetpos(): void {
    navigator.clipboard
      .writeText(command)
      .then(() => setMessage('Setpos copiado. Pégalo en la consola de CS2, en una partida con sv_cheats 1.'))
      .catch(() => setMessage('No se pudo copiar. Selecciona el texto del setpos y cópialo a mano.'));
  }

  function confirmTechnique(): void {
    patchLibraryLineup(lineup.id, { movement: lineup.movement, jump: lineup.jump, click: lineup.click })
      .then(onChanged)
      .catch(() => setMessage('No se pudo guardar la técnica.'));
  }

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Lineup"
        icon={Crosshair}
        title={lineup.title}
        subtitle={`Actualizado el ${formatShortDate(lineup.updated_at)}`}
        fields={[
          { label: 'Mapa', value: prettyMapName(lineup.map) },
          { label: 'Lado', value: lineup.side },
          { label: 'Granada', value: <GrenadeName kind={lineup.grenade} /> },
          { label: 'Desde', value: calloutLabel(lineup.map, lineup.throw_callout) || <EmptyCell label="Sin callout" /> },
          { label: 'Cae en', value: calloutLabel(lineup.map, lineup.landing_callout) || <EmptyCell label="Sin callout" /> },
          { label: 'Técnica', value: <TechniqueText lineup={lineup} /> },
          { label: 'Estado', value: <StatusBadge status={lineup.status} /> },
          { label: 'Etiquetas', value: lineup.tags.length > 0 ? lineup.tags.join(', ') : <EmptyCell label="Sin etiquetas" /> },
        ]}
        actions={
          <>
            <Button type="button" size="sm" disabled={practice.state.kind === 'busy'} onClick={() => practice.launch([lineupSpot(lineup)])}>
              <Crosshair aria-hidden />
              Practicar en CS2
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={copySetpos}>
              <Copy aria-hidden />
              Copiar setpos
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setEditing(true)}>
              <Pencil aria-hidden />
              Editar
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => setDeleting(true)}>
              <Trash2 aria-hidden />
              Eliminar
            </Button>
            <Button asChild variant="ghost" size="sm">
              <Link href={LINEUPS_HREF}>
                <ArrowLeft aria-hidden />
                Toda la Libreta
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
      <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
      {lineup.technique_estimated ? (
        <div className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-surface-2 px-3 py-2 text-body-sm text-fg-2">
          <p className="min-w-0 flex-1">
            Studio estimó cómo se lanza (movimiento y salto) con las posiciones de la demo, que se leen 8 veces por segundo, y supuso clic
            izquierdo. Pruébalo en CS2 y confirma la técnica o corrígela.
          </p>
          <Button type="button" variant="outline" size="xs" onClick={confirmTechnique}>
            Es correcta
          </Button>
          <Button type="button" variant="ghost" size="xs" onClick={() => setEditing(true)}>
            Corregir
          </Button>
        </div>
      ) : null}
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
        <div className="flex flex-col gap-5">
          <section className="flex flex-col gap-2">
            <h2 className="text-body font-semibold text-fg-1">En la consola</h2>
            <code className="block rounded-sm bg-surface-3 px-2 py-1.5 font-mono text-body-sm break-all text-fg-1">{command}</code>
            <p className="text-meta text-fg-3">
              Te pone donde se lanza y apuntando adonde hay que apuntar. Necesita sv_cheats 1; Practicar en CS2 lo prepara todo por ti.
            </p>
          </section>
          {lineup.source ? (
            <section className="flex flex-col gap-2">
              <h2 className="text-body font-semibold text-fg-1">De dónde sale</h2>
              <p className="text-body-sm text-fg-2">Studio lo leyó de la ronda {lineup.source.round} de una partida de la biblioteca.</p>
              <div>
                <Button asChild variant="outline" size="sm">
                  <Link href={evidencePath(lineup.source)}>
                    <Play aria-hidden />
                    Ver la ronda
                  </Link>
                </Button>
              </div>
            </section>
          ) : null}
          <LineupNotes key={lineup.updated_at} lineup={lineup} onSaved={onChanged} />
        </div>
        {lineup.source && lineup.landing ? (
          <section aria-label="Radar" className="flex flex-col gap-2">
            <LineupRadar matchId={lineup.source.match_id} side={lineup.side} origin={lineup.origin} landing={lineup.landing} />
          </section>
        ) : null}
      </div>
      {editing ? <EditLineupDialog lineup={lineup} onClose={() => setEditing(false)} onSaved={onChanged} /> : null}
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Eliminar {lineup.title}</DialogTitle>
            <DialogDescription>Se borra el lineup de la Libreta. No se puede deshacer; si solo quieres sacarlo de la lista, pásalo a Archivo.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleting(false)}>
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => {
                deleteLibraryLineup(lineup.id)
                  .then(() => router.push(LINEUPS_HREF))
                  .catch(() => {
                    setDeleting(false);
                    setMessage('No se pudo eliminar el lineup.');
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

function LineupNotes({ lineup, onSaved }: { lineup: LibraryLineup; onSaved: () => void }): ReactNode {
  const [notes, setNotes] = useState(lineup.notes);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await patchLibraryLineup(lineup.id, { notes });
      onSaved();
    } catch {
      setError('No se pudieron guardar las notas.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={(event) => void save(event)} className="flex max-w-3xl flex-col gap-2">
      <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
        Notas
        <Textarea
          value={notes}
          maxLength={4000}
          rows={4}
          placeholder="Por ejemplo: apunta a la esquina de la antena con la mira."
          onChange={(event) => setNotes(event.target.value)}
        />
      </label>
      {error ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
      <div>
        <Button type="submit" size="sm" disabled={busy || notes.trim() === lineup.notes.trim()}>
          Guardar notas
        </Button>
      </div>
    </form>
  );
}
