'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import type { TeamGrenadeKind } from '@/lib/api/grenades';
import type { LibretaStatus } from '@/lib/api/libreta';
import { createLibraryLineup } from '@/lib/api/lineups';
import type { TacticalSide } from '@/lib/api/tactical';
import { lineupHref } from '@/lib/clips/routes';
import { parseSetpos, setposCommand } from '@/lib/libreta/lineups';
import { parseTags } from '@/lib/libreta/status';
import { loadErrorMessage } from '@/lib/load-error';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';
import { GrenadeField, TechniqueFields, type LineupTechnique } from './lineup-fields';
import { MapField, SideField, TagsField, TitleField, mapChoices } from './play-fields';
import { StatusField } from './status-badge';
import { useDiscardGuard } from './discard-guard';

/** A lineup written by hand from what `getpos` prints in the CS2 console. Saving opens it. */
export function NewLineupDialog({ knownMaps = [], onClose }: { knownMaps?: readonly string[]; onClose: () => void }): ReactNode {
  const router = useRouter();
  const maps = mapChoices(knownMaps);
  const [title, setTitle] = useState('');
  const [map, setMap] = useState(maps[0] ?? 'de_mirage');
  const [side, setSide] = useState<TacticalSide>('T');
  const [grenade, setGrenade] = useState<TeamGrenadeKind>('smoke');
  const [setpos, setSetpos] = useState('');
  const [technique, setTechnique] = useState<LineupTechnique>({ movement: 'standing', jump: false, click: 'left' });
  const [status, setStatus] = useState<LibretaStatus>('draft');
  const [tags, setTags] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const snapshot = JSON.stringify([title, map, side, grenade, setpos, technique, status, tags]);
  const [initial] = useState(snapshot);
  const guard = useDiscardGuard({ dirty: snapshot !== initial, busy, onClose });
  const aim = parseSetpos(setpos);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (aim === null) return;
    setBusy(true);
    setError(null);
    try {
      const lineup = await createLibraryLineup({
        title: title.trim(),
        map,
        side,
        grenade,
        origin: aim.origin,
        pitch: aim.pitch,
        yaw: aim.yaw,
        ...technique,
        status,
        tags: parseTags(tags),
      });
      router.push(lineupHref(lineup.id));
    } catch (cause: unknown) {
      setError(loadErrorMessage({ error: cause, lead: 'No se pudo crear el lineup.' }));
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={guard.onOpenChange}>
      <DialogContent className="sm:max-w-lg" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Nuevo lineup</DialogTitle>
            <DialogDescription>
              En CS2, ponte donde lanzas la granada, apunta y escribe getpos en la consola. Pega aquí lo que responde.
            </DialogDescription>
          </DialogHeader>
          <TitleField value={title} onChange={setTitle} />
          <div className="grid gap-3 sm:grid-cols-3">
            <MapField value={map} onChange={setMap} maps={maps} />
            <SideField value={side} onChange={setSide} />
            <GrenadeField value={grenade} onChange={setGrenade} />
          </div>
          <Field
            label="Posición y mira"
            hint={aim !== null ? <>Leído: <code className="font-mono text-fg-2">{setposCommand(aim)}</code></> : undefined}
            error={aim === null && setpos.trim() !== '' ? 'No se reconoce. Tiene que llevar setpos con tres números y setang con al menos dos.' : undefined}
          >
            {(control) => (
              <Textarea
                {...control}
                value={setpos}
                rows={2}
                maxLength={400}
                spellCheck={false}
                placeholder="setpos -1012.03 -2233.97 -167.97;setang 7.38 -68.11 0.00"
                onChange={(event) => setSetpos(event.target.value)}
                className="font-mono"
              />
            )}
          </Field>
          <TechniqueFields value={technique} onChange={setTechnique} />
          <div className="grid gap-3 sm:grid-cols-2">
            <StatusField value={status} onChange={setStatus} />
            <TagsField value={tags} onChange={setTags} />
          </div>
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || title.trim() === '' || aim === null} loading={busy}>
              Crear
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
      {guard.confirm}
    </Dialog>
  );
}
