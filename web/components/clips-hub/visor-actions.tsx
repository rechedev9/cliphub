'use client';

import { useRef, useState, useSyncExternalStore, type ReactElement, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, FolderOpen, Music, Pencil, Trash2, UploadCloud, type LucideIcon } from 'lucide-react';
import { api } from '@/lib/api';
import { streamsApi } from '@/lib/api/streams';
import { VISOR_ACTION, type VisorAction } from '@/lib/clips/visor-actions';
import type { VisorEntry } from '@/lib/clips/visor-queue';
import { cn } from '@/lib/utils';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
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

const ACTION_CLASS = 'h-11 w-full justify-start gap-3 px-3 text-body-sm motion-safe:active:scale-[0.97] max-md:w-11 max-md:justify-center max-md:px-0 [&_svg:not([class*=size-])]:size-5';

const NARROW_QUERY = '(width < 48rem)';

function subscribeNarrow(onChange: () => void): () => void {
  const query = window.matchMedia(NARROW_QUERY);
  query.addEventListener('change', onChange);
  return () => query.removeEventListener('change', onChange);
}

function useNarrow(): boolean {
  return useSyncExternalStore(subscribeNarrow, () => window.matchMedia(NARROW_QUERY).matches, () => false);
}

export type VisorActionsProps = {
  entry: VisorEntry;
  onChange: () => void;
  onDeleted: () => void;
  className?: string;
};

export function VisorActions({ entry, onChange, onDeleted, className }: VisorActionsProps): ReactNode {
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [musicOpen, setMusicOpen] = useState(false);
  const opener = useRef<HTMLButtonElement | null>(null);
  const narrow = useNarrow();
  const { item, source } = entry;

  const returnFocus = (event: Event): void => {
    event.preventDefault();
    opener.current?.focus({ preventScroll: true });
  };

  const download = (): void => {
    if (source.kind === 'demo' && source.loteShort !== undefined) downloadMP4(source.loteShort.url, entry.title);
    else if (source.kind === 'demo') downloadVideoMP4(source.video);
    else downloadMP4(item.playbackUrl, item.title);
  };
  const remove = (): Promise<void> => {
    if (source.kind === 'stream') return streamsApi.deleteVideo(item.jobId, item.variant, source.clipId);
    return source.loteShort === undefined ? api.deleteVideo(source.video.id) : api.deleteLoteShort(source.video.id, source.loteShort.name);
  };

  const press = (action: VisorAction, button: HTMLButtonElement): void => {
    opener.current = button;
    if (action.kind === VISOR_ACTION.music) setMusicOpen(true);
    if (action.kind === VISOR_ACTION.download) download();
    if (action.kind === VISOR_ACTION.delete) setDeleteOpen(true);
  };

  return (
    <div role="group" aria-label={`Acciones de ${entry.title}`} className={cn('flex w-48 flex-col gap-1 max-md:w-auto max-md:items-center', className)}>
      {entry.rendering ? (
        <StatusTag tone="primary" dot className="mb-2 self-start max-md:mb-1 max-md:self-center">
          <span role="status">Renderizando cambios</span>
        </StatusTag>
      ) : null}
      <div className="flex flex-col gap-1 max-md:flex-row max-md:flex-wrap max-md:justify-center">
        {entry.actions.map((action) => {
          const Icon = ICON[action.kind];
          const tone = action.kind === VISOR_ACTION.delete ? 'text-fg-2 hover:bg-destructive/12 hover:text-destructive' : undefined;
          const label = <span className="max-md:sr-only">{action.label}</span>;
          if ('href' in action) {
            return (
              <ActionTip key={action.kind} label={action.label} shown={narrow}>
                <Button asChild variant="ghost" className={ACTION_CLASS}>
                  <Link href={action.href}>
                    <Icon aria-hidden />
                    {label}
                  </Link>
                </Button>
              </ActionTip>
            );
          }
          return (
            <ActionTip key={action.kind} label={action.label} shown={narrow}>
              <Button type="button" variant="ghost" className={cn(ACTION_CLASS, tone)} onClick={(event) => press(action, event.currentTarget)}>
                <Icon aria-hidden />
                {label}
              </Button>
            </ActionTip>
          );
        })}
      </div>
      <DeleteVideoDialog title={entry.title} onConfirm={remove} open={deleteOpen} onOpenChange={setDeleteOpen} onDeleted={() => { onDeleted(); onChange(); }} onCloseAutoFocus={returnFocus} />
      {source.kind === 'demo' && entry.actions.some((action) => action.kind === VISOR_ACTION.music) ? (
        <LibraryMusicDialog open={musicOpen} video={source.video} onOpenChange={setMusicOpen} onApplied={onChange} onCloseAutoFocus={returnFocus} />
      ) : null}
    </div>
  );
}

function ActionTip({ label, shown, children }: { label: string; shown: boolean; children: ReactElement }): ReactNode {
  const [open, setOpen] = useState(false);
  return (
    <Tooltip open={shown && open} onOpenChange={setOpen}>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}
