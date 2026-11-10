'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { ListOrdered, Save, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { isLibraryOffline } from '@/lib/api/library';
import {
  PATTERN_NAME_MAX,
  createSavedPattern,
  deleteSavedPattern,
  listSavedPatterns,
  savedPatternRounds,
  type PatternDefinition,
  type PatternRunResponse,
  type PatternScope,
  type SavedPattern,
  type TeamPatternScope,
} from '@/lib/api/patterns';
import { TREND_LABEL, ofRoundsLabel, percentLabel, timedKind } from '@/lib/analysis/pattern-finder';
import { prettyMapName } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { PatternResults } from './pattern-results';

type Reach = 'team' | 'library';

type ListProps = {
  map: string;
  teamId: string;
  /** Kept stable by the owner: a new object reads the list again. */
  scope: TeamPatternScope;
  /** Bumped after a save, so the list reads again. */
  version: number;
  onOpen: (pattern: SavedPattern) => void;
};

/** The saved patterns of the map with their figures, over this team or detected over the whole library. */
export function SavedPatterns({ map, teamId, scope, version, onOpen }: ListProps): ReactNode {
  const [reach, setReach] = useState<Reach>('team');
  const [list, setList] = useState<SavedPattern[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [removed, setRemoved] = useState(0);
  const [confirming, setConfirming] = useState<SavedPattern | null>(null);
  const [wide, setWide] = useState<SavedPattern | null>(null);

  useEffect(() => {
    let active = true;
    setFailed(false);
    const read = reach === 'team' ? listSavedPatterns({ map, teamId, scope, stats: true }) : listSavedPatterns({ map, stats: true });
    read
      .then((patterns) => {
        if (active) setList(patterns);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [map, teamId, reach, scope, version, removed]);

  const remove = useCallback(async (pattern: SavedPattern) => {
    try {
      await deleteSavedPattern(pattern.id);
      setConfirming(null);
      setRemoved((n) => n + 1);
    } catch (error) {
      toast.error(isLibraryOffline(error) ? 'El servicio local no responde.' : `No se pudo borrar «${pattern.name}».`);
    }
  }, []);

  return (
    <section aria-labelledby="patrones-guardados" className="flex flex-col gap-2 border-t border-border pt-3">
      <h2 id="patrones-guardados" className="text-body font-semibold text-fg-1">
        Patrones guardados de {prettyMapName(map)}
      </h2>
      <Segmented<Reach>
        label="Rondas que leen"
        value={reach}
        options={[
          { value: 'team', label: 'Este equipo' },
          { value: 'library', label: 'Toda la biblioteca' },
        ]}
        onChange={setReach}
      />
      {failed ? <p className="text-body-sm text-fg-2">No se pudieron leer los patrones guardados.</p> : null}
      {list !== null && list.length === 0 ? <p className="text-body-sm text-fg-2">Todavía no hay patrones guardados en este mapa.</p> : null}
      <ul className="flex flex-col gap-1.5">
        {(list ?? []).map((pattern) => (
          <li key={pattern.id} className="flex items-start gap-2 rounded-md border border-border bg-surface-1 px-2.5 py-2">
            <button type="button" onClick={() => onOpen(pattern)} className="flex min-w-0 flex-1 flex-col items-start gap-0.5 text-left outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
              <span className="w-full truncate text-body-sm font-medium text-fg-1">{pattern.name}</span>
              {pattern.stats === undefined ? null : (
                <span className="text-meta tabular-nums text-fg-2">
                  {ofRoundsLabel(pattern.stats.matched, pattern.stats.rounds)} · gana {pattern.stats.matched === 0 ? 'sin rondas' : percentLabel(pattern.stats.win_rate)} ·
                  últimos 30 días {pattern.stats.recent_matched} · tendencia {TREND_LABEL[pattern.stats.trend].toLowerCase()}
                </span>
              )}
            </button>
            {pattern.team_id === undefined ? (
              <Button type="button" variant="ghost" size="icon-xs" aria-label={`Rondas de ${pattern.name} en toda la biblioteca`} onClick={() => setWide(pattern)}>
                <ListOrdered aria-hidden />
              </Button>
            ) : null}
            <Button type="button" variant="ghost" size="icon-xs" aria-label={`Borrar el patrón ${pattern.name}`} onClick={() => setConfirming(pattern)}>
              <Trash2 aria-hidden />
            </Button>
          </li>
        ))}
      </ul>
      <Dialog open={confirming !== null} onOpenChange={(next) => !next && setConfirming(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>¿Borrar «{confirming?.name}»?</DialogTitle>
            <DialogDescription>Deja de buscarse en las partidas nuevas. No se puede deshacer.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="ghost" size="sm" onClick={() => setConfirming(null)}>
              Cancelar
            </Button>
            <Button type="button" variant="destructive" size="sm" onClick={() => confirming !== null && void remove(confirming)}>
              Borrar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      {wide === null ? null : <LibraryRounds pattern={wide} onClose={() => setWide(null)} />}
    </section>
  );
}

type Rounds = { kind: 'loading' } | { kind: 'ready'; data: PatternRunResponse } | { kind: 'failed' };

/** A library-wide pattern's own rounds, over every team of every analysed match of its map. */
function LibraryRounds({ pattern, onClose }: { pattern: SavedPattern; onClose: () => void }): ReactNode {
  const [rounds, setRounds] = useState<Rounds>({ kind: 'loading' });
  useEffect(() => {
    let active = true;
    savedPatternRounds(pattern.id)
      .then((data) => {
        if (active) setRounds({ kind: 'ready', data });
      })
      .catch(() => {
        if (active) setRounds({ kind: 'failed' });
      });
    return () => {
      active = false;
    };
  }, [pattern.id]);
  return (
    <Dialog open onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="max-h-[85dvh] max-w-lg overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{pattern.name}</DialogTitle>
          <DialogDescription>Sus rondas en toda la biblioteca de {prettyMapName(pattern.map)}, de los dos equipos.</DialogDescription>
        </DialogHeader>
        {rounds.kind === 'loading' ? <Skeleton className="h-60 w-full" /> : null}
        {rounds.kind === 'failed' ? <p className="text-body-sm text-fg-2">No se pudieron leer sus rondas.</p> : null}
        {rounds.kind === 'ready' ? <PatternResults result={rounds.data.result} timed={timedKind(pattern.pattern)} title={`${pattern.name} en ${prettyMapName(pattern.map)}`} /> : null}
      </DialogContent>
    </Dialog>
  );
}

type SaveProps = {
  map: string;
  teamId: string;
  teamName: string;
  pattern: PatternDefinition;
  scope: PatternScope;
  onSaved: () => void;
};

/** "Guardar patrón": a name and whether the pattern belongs to this team or reads the whole library. */
export function SavePatternButton({ map, teamId, teamName, pattern, scope, onSaved }: SaveProps): ReactNode {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [reach, setReach] = useState<Reach>('team');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save(): Promise<void> {
    const trimmed = name.trim();
    if (trimmed === '') {
      setError('Ponle un nombre al patrón.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await createSavedPattern(reach === 'team' ? { name: trimmed, map, team_id: teamId, pattern, scope } : { name: trimmed, map, pattern, scope });
      toast.success(`Patrón «${trimmed}» guardado`);
      setOpen(false);
      setName('');
      onSaved();
    } catch (failure) {
      setError(isLibraryOffline(failure) ? 'El servicio local no responde. Vuelve a intentarlo.' : 'No se pudo guardar el patrón.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Button type="button" variant="outline" size="sm" onClick={() => setOpen(true)}>
        <Save aria-hidden />
        Guardar patrón
      </Button>
      <Dialog open={open} onOpenChange={(next) => !busy && setOpen(next)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Guardar patrón</DialogTitle>
            <DialogDescription>Studio lo vuelve a buscar en cada partida nueva de {prettyMapName(map)} y te da sus rondas, cuántas gana y su tendencia.</DialogDescription>
          </DialogHeader>
          <form
            className="flex flex-col gap-3"
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <Input aria-label="Nombre" placeholder="Nombre del patrón" maxLength={PATTERN_NAME_MAX} value={name} onChange={(event) => setName(event.currentTarget.value)} autoFocus />
            <Segmented<Reach>
              label="De quién es"
              value={reach}
              options={[
                { value: 'team', label: teamName === '' ? 'Este equipo' : teamName },
                { value: 'library', label: 'Toda la biblioteca' },
              ]}
              onChange={setReach}
            />
            {error === null ? null : <p className="text-body-sm text-destructive">{error}</p>}
            <DialogFooter>
              <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => setOpen(false)}>
                Cancelar
              </Button>
              <Button type="submit" size="sm" disabled={busy}>
                Guardar
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
