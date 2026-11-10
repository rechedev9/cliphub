'use client';

import { useEffect, useId, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { isLibraryOffline, listLibraryPlays } from '@/lib/api/library';
import { PATTERN_CATEGORY_MAX, PATTERN_NAME_MAX, updateSavedPattern, type PatternPatch, type SavedPattern } from '@/lib/api/patterns';
import type { LibraryPlay } from '@/lib/api/stratbook';
import { prettyMapName } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const NO_PLAY = 'none';

type Reach = 'team' | 'library';

/** A text field for the category, offering the ones already in use. */
export function CategoryInput({ value, known, onChange }: { value: string; known: readonly string[]; onChange: (value: string) => void }): ReactNode {
  const listId = useId();
  return (
    <Field label="Categoría" hint="Agrupa y filtra la lista, como Ejecuciones o Defensas.">
      {(control) => (
        <>
          <Input {...control} list={listId} maxLength={PATTERN_CATEGORY_MAX} placeholder="Sin categoría" value={value} onChange={(event) => onChange(event.currentTarget.value)} />
          <datalist id={listId}>
            {known.map((entry) => (
              <option key={entry} value={entry} />
            ))}
          </datalist>
        </>
      )}
    </Field>
  );
}

/** A select of the map's Libreta plays; empty unlinks. */
export function PlaySelect({ map, value, onChange }: { map: string; value: string; onChange: (playId: string) => void }): ReactNode {
  const [plays, setPlays] = useState<LibraryPlay[]>([]);
  useEffect(() => {
    let active = true;
    listLibraryPlays({ map })
      .then((list) => {
        if (active) setPlays(list);
      })
      .catch(() => {
        if (active) setPlays([]);
      });
    return () => {
      active = false;
    };
  }, [map]);
  return (
    <Field label="Jugada de la Libreta" hint="La jugada muestra este patrón y el patrón abre la jugada.">
      {(control) => (
        <Select value={value === '' ? NO_PLAY : value} onValueChange={(next) => onChange(next === NO_PLAY ? '' : next)}>
          <SelectTrigger {...control}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={NO_PLAY}>Sin jugada</SelectItem>
            {plays.map((play) => (
              <SelectItem key={play.id} value={play.id}>
                {play.title}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </Field>
  );
}

type EditProps = {
  pattern: SavedPattern;
  /** The team whose view is open; without one the pattern keeps its owner. */
  teamId?: string;
  teamName?: string;
  /** Categories already in use, offered as suggestions. */
  categories: readonly string[];
  onClose: () => void;
  onSaved: (pattern: SavedPattern) => void;
};

function patchOf(pattern: SavedPattern, next: { name: string; category: string; playId: string; reach: Reach; teamId?: string }): PatternPatch {
  const patch: PatternPatch = {};
  if (next.name !== pattern.name) patch.name = next.name;
  if (next.category !== pattern.category) patch.category = next.category;
  if (next.playId === '' && pattern.play_id !== undefined) patch.clear_play = true;
  if (next.playId !== '' && next.playId !== pattern.play_id) patch.play_id = next.playId;
  if (next.reach === 'library' && pattern.team_id !== undefined) patch.clear_team = true;
  if (next.reach === 'team' && next.teamId !== undefined && pattern.team_id !== next.teamId) patch.team_id = next.teamId;
  return patch;
}

/** "Editar": the name, category, owner and linked play of a saved pattern. */
export function PatternEditDialog({ pattern, teamId, teamName = '', categories, onClose, onSaved }: EditProps): ReactNode {
  const [name, setName] = useState(pattern.name);
  const [category, setCategory] = useState(pattern.category);
  const [playId, setPlayId] = useState(pattern.play_id ?? '');
  const [reach, setReach] = useState<Reach>(pattern.team_id === undefined ? 'library' : 'team');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save(): Promise<void> {
    const trimmed = name.trim();
    if (trimmed === '') {
      setError('Ponle un nombre al patrón.');
      return;
    }
    const patch = patchOf(pattern, { name: trimmed, category: category.trim(), playId, reach, teamId: teamId ?? pattern.team_id });
    if (Object.keys(patch).length === 0) {
      onClose();
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const saved = await updateSavedPattern(pattern.id, patch);
      toast.success(`Patrón «${saved.name}» guardado`);
      onSaved(saved);
    } catch (failure) {
      setError(isLibraryOffline(failure) ? 'El servicio local no responde. Vuelve a intentarlo.' : 'No se pudieron guardar los cambios.');
    } finally {
      setBusy(false);
    }
  }

  const canOwn = teamId !== undefined || pattern.team_id !== undefined;
  return (
    <Dialog open onOpenChange={(next) => !next && !busy && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Editar «{pattern.name}»</DialogTitle>
          <DialogDescription>Patrón de {prettyMapName(pattern.map)}. Para cambiar sus bloques, ábrelo en el buscador y guarda los cambios.</DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            void save();
          }}
        >
          <Field label="Nombre">
            {(control) => <Input {...control} maxLength={PATTERN_NAME_MAX} value={name} onChange={(event) => setName(event.currentTarget.value)} autoFocus />}
          </Field>
          <CategoryInput value={category} known={categories} onChange={setCategory} />
          {canOwn ? (
            <Segmented<Reach>
              label="De quién es"
              value={reach}
              options={[
                { value: 'team', label: teamName === '' ? 'Este equipo' : teamName },
                { value: 'library', label: 'Toda la biblioteca' },
              ]}
              onChange={setReach}
            />
          ) : null}
          <PlaySelect map={pattern.map} value={playId} onChange={setPlayId} />
          {error === null ? null : <p className="text-body-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" size="sm" disabled={busy}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
