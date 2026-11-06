'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { createLibraryPlay } from '@/lib/api/library';
import type { LibretaStatus } from '@/lib/api/libreta';
import type { LibraryPlay } from '@/lib/api/stratbook';
import type { TacticalSide, TacticalSite } from '@/lib/api/tactical';
import { playHref } from '@/lib/clips/routes';
import { parseTags } from '@/lib/libreta/status';
import { playKinds } from '@/lib/stratbook/plays';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Field, KindField, MapField, NO_TEAM, SideField, SiteField, TagsField, TeamField, TitleField, mapChoices, useLibraryTeams } from './play-fields';
import { StatusSelect } from './status-badge';

/** What the dialog opens with: the Tabla passes the cell's map, side, kind and team. */
export type NewPlayPreset = { map?: string; side?: TacticalSide; kind?: string; teamId?: string };

/**
 * A play written by hand: a name, map, side and kind, with no grenades yet.
 * Saving opens it, unless `onCreated` keeps the user where they are.
 */
export function NewPlayDialog({
  preset = {},
  knownMaps = [],
  onClose,
  onCreated,
}: {
  preset?: NewPlayPreset;
  knownMaps?: readonly string[];
  onClose: () => void;
  onCreated?: (play: LibraryPlay) => void;
}): ReactNode {
  const router = useRouter();
  const teams = useLibraryTeams();
  const maps = mapChoices(knownMaps);
  const [title, setTitle] = useState('');
  const [map, setMap] = useState(preset.map ?? maps[0] ?? 'de_mirage');
  const [side, setSide] = useState<TacticalSide>(preset.side ?? 'T');
  const [kind, setKind] = useState(preset.kind ?? playKinds(preset.side ?? 'T')[0] ?? 'default');
  const [site, setSite] = useState<TacticalSite>('none');
  const [teamId, setTeamId] = useState(preset.teamId ?? NO_TEAM);
  const [status, setStatus] = useState<LibretaStatus>('draft');
  const [tags, setTags] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function changeSide(next: TacticalSide): void {
    setSide(next);
    if (!playKinds(next).includes(kind)) setKind(playKinds(next)[0] ?? 'default');
  }

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const play = await createLibraryPlay({
        title: title.trim(),
        team_id: teamId === NO_TEAM ? null : teamId,
        map,
        side,
        kind,
        site,
        origin: 'manual',
        positions_seconds: 0,
        grenades: [],
        positions: [],
        notes: '',
        examples: [],
        status,
        tags: parseTags(tags),
      });
      if (onCreated) {
        onCreated(play);
        onClose();
      } else {
        router.push(playHref(play.id));
      }
    } catch {
      setError('No se pudo crear la jugada.');
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Nueva jugada</DialogTitle>
            <DialogDescription>
              Escríbela con su nombre y su tipo. Las granadas y las posiciones llegan al guardar una ronda desde la repetición 2D.
            </DialogDescription>
          </DialogHeader>
          <TitleField value={title} onChange={setTitle} />
          <div className="grid gap-3 sm:grid-cols-2">
            <MapField value={map} onChange={setMap} maps={maps} />
            <SideField value={side} onChange={changeSide} />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <KindField side={side} value={kind} onChange={setKind} />
            <SiteField value={site} onChange={setSite} />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <TeamField value={teamId} onChange={setTeamId} teams={teams} />
            <Field label="Estado">
              <StatusSelect value={status} onChange={setStatus} />
            </Field>
          </div>
          <TagsField value={tags} onChange={setTags} />
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || title.trim() === ''}>
              Crear
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
