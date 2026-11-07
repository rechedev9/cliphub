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
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';
import { GrenadeField, TechniqueFields, type LineupTechnique } from './lineup-fields';
import { Field, MapField, SideField, TagsField, TitleField, mapChoices } from './play-fields';
import { StatusSelect } from './status-badge';

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
    } catch {
      setError('No se pudo crear el lineup.');
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
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
          <Field label="Posición y mira">
            <Textarea
              value={setpos}
              rows={2}
              maxLength={400}
              spellCheck={false}
              placeholder="setpos -1012.03 -2233.97 -167.97;setang 7.38 -68.11 0.00"
              onChange={(event) => setSetpos(event.target.value)}
              className="font-mono"
            />
            {aim !== null ? (
              <span className="text-meta text-fg-3">
                Leído: <code className="font-mono text-fg-2">{setposCommand(aim)}</code>
              </span>
            ) : null}
            {aim === null && setpos.trim() !== '' ? (
              <span role="alert" className="text-meta text-destructive">
                No se reconoce. Tiene que llevar setpos con tres números y setang con al menos dos.
              </span>
            ) : null}
          </Field>
          <TechniqueFields value={technique} onChange={setTechnique} />
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Estado">
              <StatusSelect value={status} onChange={setStatus} />
            </Field>
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
            <Button type="submit" disabled={busy || title.trim() === '' || aim === null}>
              Crear
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
