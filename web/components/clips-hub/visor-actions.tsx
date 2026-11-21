'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, FolderOpen, Music, Pencil, Trash2, UploadCloud, type LucideIcon } from 'lucide-react';
import { api } from '@/lib/api';
import { streamsApi } from '@/lib/api/streams';
import { VISOR_ACTION, type VisorAction } from '@/lib/clips/visor-actions';
import type { VisorEntry } from '@/lib/clips/visor-queue';
import { cn } from '@/lib/utils';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { DeleteVideoDialog } from '@/components/videos/delete-video-button';
import { LibraryMusicDialog } from '@/components/videos/library-music-dialog';
import { downloadMP4, downloadVideoMP4 } from '@/components/clips-hub/output-item';

const ICON = {
  [VISOR_ACTION.edit]: Pencil,
  [VISOR_ACTION.music]: Music,
  [VISOR_ACTION.publish]: UploadCloud,
  [VISOR_ACTION.openProject]: FolderOpen,
  [VISOR_ACTION.download]: Download,
  [VISOR_ACTION.delete]: Trash2,
} as const satisfies Record<VisorAction['kind'], LucideIcon>;

const ACTION_CLASS = 'h-11 w-full justify-start gap-3 px-3 text-body-sm motion-safe:active:scale-[0.97] [&_svg:not([class*=size-])]:size-5';

export type VisorActionsProps = {
  entry: VisorEntry;
  onChange: () => void;
  onDeleting: () => void;
  className?: string;
};

export function VisorActions({ entry, onChange, onDeleting, className }: VisorActionsProps): ReactNode {
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [musicOpen, setMusicOpen] = useState(false);
  const { item, source } = entry;

  const download = (): void => {
    if (source.kind === 'demo' && source.loteShort !== undefined) downloadMP4(source.loteShort.url, entry.title);
    else if (source.kind === 'demo') downloadVideoMP4(source.video);
    else downloadMP4(item.playbackUrl, item.title);
  };
  const remove = (): Promise<void> => {
    onDeleting();
    if (source.kind === 'stream') return streamsApi.deleteVideo(item.jobId, item.variant, source.clipId);
    return source.loteShort === undefined ? api.deleteVideo(source.video.id) : api.deleteLoteShort(source.video.id, source.loteShort.name);
  };

  const press = (action: VisorAction): void => {
    if (action.kind === VISOR_ACTION.music) setMusicOpen(true);
    if (action.kind === VISOR_ACTION.download) download();
    if (action.kind === VISOR_ACTION.delete) setDeleteOpen(true);
  };

  return (
    <div role="group" aria-label={`Acciones de ${entry.title}`} className={cn('flex w-48 flex-col gap-1', className)}>
      {entry.rendering ? (
        <StatusTag tone="primary" dot className="mb-2 self-start">
          <span role="status">Renderizando cambios</span>
        </StatusTag>
      ) : null}
      {entry.actions.map((action) => {
        const Icon = ICON[action.kind];
        const tone = action.kind === VISOR_ACTION.delete ? 'text-fg-2 hover:bg-destructive/12 hover:text-destructive' : undefined;
        if ('href' in action) {
          return (
            <Button key={action.kind} asChild variant="ghost" className={ACTION_CLASS}>
              <Link href={action.href}>
                <Icon aria-hidden />
                {action.label}
              </Link>
            </Button>
          );
        }
        return (
          <Button key={action.kind} type="button" variant="ghost" className={cn(ACTION_CLASS, tone)} onClick={() => press(action)}>
            <Icon aria-hidden />
            {action.label}
          </Button>
        );
      })}
      <DeleteVideoDialog title={entry.title} onConfirm={remove} open={deleteOpen} onOpenChange={setDeleteOpen} onDeleted={onChange} />
      {source.kind === 'demo' && entry.actions.some((action) => action.kind === VISOR_ACTION.music) ? (
        <LibraryMusicDialog open={musicOpen} video={source.video} onOpenChange={setMusicOpen} onApplied={onChange} />
      ) : null}
    </div>
  );
}
