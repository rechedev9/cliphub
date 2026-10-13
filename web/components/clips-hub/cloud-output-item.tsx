'use client';

import { memo, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Cloud, Download, FolderOpen, Link2, Monitor, Play, RotateCcw, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { cloudApi } from '@/lib/api/cloud';
import { PLAYBACK_REVIEW, PLAYBACK_SOURCE, type MediaPlaybackItem } from '@/lib/api/playback';
import { sameHubProps } from '@/lib/clips/hub';
import { cloudDeviceLinked } from '@/lib/cloud/account-view';
import { CAPTURE_TARGET, produceShortHref } from '@/lib/cloud/capture-target';
import type { CloudJob } from '@/lib/cloud/parse';
import { cloudJobViewOn, type CloudJobTone } from '@/lib/cloud/view';
import { streamVideoFilename } from '@/lib/streams/download';
import { cn } from '@/lib/utils';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { CloudLinkDialog } from '@/components/cloud/cloud-link-dialog';
import { MediaPlayer } from '@/components/studio/media-player';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';

type RowAction = 'cancel' | 'remove';

/** The question asked before an action that gives something up; the consequence itself comes from the job view. */
const CONFIRM_COPY = {
  cancel: { title: '¿Cancelar este vídeo?', confirm: 'Cancelar vídeo', keep: 'Seguir esperando' },
  remove: { title: '¿Quitar este vídeo?', confirm: 'Quitar', keep: 'Conservar' },
} as const satisfies Record<RowAction, { title: string; confirm: string; keep: string }>;

const BORDER_CLASS = {
  queue: 'border-border',
  working: 'border-border-accent',
  ready: 'border-success/45',
  failed: 'border-destructive/45',
  neutral: 'border-border',
} as const satisfies Record<CloudJobTone, string>;

const BAR_CLASS = {
  queue: 'text-fg-3',
  working: 'text-primary',
  ready: 'text-success',
  failed: 'text-destructive',
  neutral: 'text-fg-3',
} as const satisfies Record<CloudJobTone, string>;

const TAG_TONE = {
  queue: 'neutral',
  working: 'primary',
  ready: 'success',
  failed: 'danger',
  neutral: 'neutral',
} as const satisfies Record<CloudJobTone, StatusTagTone>;

const SAVED_POLL_MS = 1500;

type DownloadsBridge = { isSaved: (url: string) => Promise<boolean>; reveal: (url: string) => Promise<boolean> };

/** The desktop bridge that knows which downloads finished; absent in a plain browser. */
function downloadsBridge(): DownloadsBridge | null {
  const value: unknown = Reflect.get(window, 'cliphubDownloads');
  if (typeof value !== 'object' || value === null) return null;
  const isSaved: unknown = Reflect.get(value, 'isSaved');
  const reveal: unknown = Reflect.get(value, 'reveal');
  if (typeof isSaved !== 'function' || typeof reveal !== 'function') return null;
  return {
    isSaved: async (url) => (await Reflect.apply(isSaved, value, [url])) === true,
    reveal: async (url) => (await Reflect.apply(reveal, value, [url])) === true,
  };
}

function absolute(url: string): string {
  return new URL(url, window.location.origin).href;
}

function playbackItem(job: CloudJob, name: string): MediaPlaybackItem {
  const variant = job.videos.find((video) => video.name === name)?.variant ?? '';
  return {
    id: `cloud:${job.id}:${name}`,
    source: PLAYBACK_SOURCE.demo,
    jobId: job.id,
    variant,
    artifactName: name,
    // A cloud job's files are written once, so the job id is a stable revision.
    revision: `cloud:${job.id}`,
    title: job.title,
    format: job.kind === 'full_demo' ? '16:9' : '9:16',
    playbackUrl: cloudApi.videoUrl(job.id, name),
    review: PLAYBACK_REVIEW.ready,
    warnings: [],
  };
}

export type CloudOutputItemProps = {
  job: CloudJob;
  onChange: () => void;
};

/** One cloud job in the hub: where it stands in the cloud and, when done, the video on this PC. */
function CloudOutputItemCard({ job, onChange }: CloudOutputItemProps): ReactNode {
  const linked = cloudDeviceLinked(useCloudAccount());
  const view = cloudJobViewOn(job, { now: Date.now(), linked });
  const [pending, setPending] = useState<RowAction | null>(null);
  // The question stays set while the dialog closes, so its text does not change mid-animation.
  const [confirm, setConfirm] = useState<{ action: RowAction; warning: string } | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const [playerOpen, setPlayerOpen] = useState(false);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  const items = view.playable.map((name) => playbackItem(job, name));
  const [first] = items;

  function settle(): void {
    setPending(null);
    setConfirmOpen(false);
  }

  function cancel(): void {
    setPending('cancel');
    void cloudApi
      .cancel(job.id)
      .then(onChange)
      .catch(() => toast('No se pudo cancelar', { description: job.title }))
      .finally(settle);
  }

  function remove(): void {
    setPending('remove');
    void cloudApi
      .remove(job.id)
      .then(onChange)
      .catch(() => toast('No se pudo quitar', { description: job.title }))
      .finally(settle);
  }

  /** Acts at once when nothing is lost; otherwise asks first, naming what is. */
  function request(action: RowAction, warning: string | null): void {
    if (warning === null) {
      if (action === 'cancel') cancel();
      else remove();
      return;
    }
    setConfirm({ action, warning });
    setConfirmOpen(true);
  }

  return (
    <>
      <div
        data-cloud-job={job.id}
        className={cn(
          'flex flex-wrap items-center gap-3 rounded-lg border bg-surface-2 px-3 py-2.5 transition-colors duration-(--dur-base)',
          BORDER_CLASS[view.tone],
        )}
      >
        <span
          aria-hidden
          className={cn('flex h-[50px] w-7 shrink-0 items-center justify-center border bg-surface-1 text-fg-3', BORDER_CLASS[view.tone])}
        >
          <Cloud className="size-4" />
        </span>

        <span className="flex min-w-[10rem] flex-1 flex-col gap-1.5">
          <span className="flex items-center gap-2">
            <span className="min-w-0 truncate font-display text-label font-bold uppercase text-fg-1">{job.title}</span>
            <StatusTag tone={TAG_TONE[view.tone]} icon={Cloud} className="shrink-0">
              Nube
            </StatusTag>
          </span>

          {view.bar ? (
            <span className={cn('studio-bar', BAR_CLASS[view.tone])}>
              <span
                className={view.percent === null ? 'studio-indeterminate' : undefined}
                style={view.percent === null ? undefined : { width: `${view.percent}%` }}
              />
            </span>
          ) : null}

          {view.failure === null ? (
            <span role="status" className="font-mono text-meta uppercase tracking-wider text-fg-3">
              {view.line}
            </span>
          ) : (
            <span role="status" className="text-body-sm text-destructive">
              {view.line}
            </span>
          )}
        </span>

        {/* Always its own line: up to three labelled actions would squeeze the title out of a half-width column. */}
        <span className="flex w-full flex-wrap items-center gap-1.5">
          {first !== undefined ? (
            <Button
              type="button"
              size="xs"
              variant="outline-primary"
              onClick={(event) => {
                setReturnFocus(event.currentTarget);
                setPlayerOpen(true);
              }}
            >
              <Play aria-hidden />
              Reproducir
            </Button>
          ) : null}
          {view.playable.map((name) => (
            <SaveVideo key={name} jobId={job.id} name={name} title={job.title} labelled={view.playable.length > 1} />
          ))}
          {view.failure?.retryCloud && job.localJobId !== '' ? (
            <Button asChild size="xs" variant="outline-primary">
              <Link href={produceShortHref(job.localJobId, CAPTURE_TARGET.cloud)}>
                <RotateCcw aria-hidden />
                Reintentar en la nube
              </Link>
            </Button>
          ) : null}
          {view.failure?.recordLocal && job.localJobId !== '' ? (
            <Button asChild size="xs" variant="outline">
              <Link href={produceShortHref(job.localJobId, CAPTURE_TARGET.local)}>
                <Monitor aria-hidden />
                Grabar en este PC
              </Link>
            </Button>
          ) : null}
          {view.needsLink ? (
            <Button type="button" size="xs" variant="outline-primary" onClick={() => setLinkOpen(true)}>
              <Link2 aria-hidden />
              Conectar cuenta
            </Button>
          ) : null}
          {view.canCancel ? (
            <Button type="button" size="xs" variant="outline" loading={pending === 'cancel'} onClick={() => request('cancel', view.cancelWarning)}>
              <X aria-hidden />
              Cancelar
            </Button>
          ) : null}
          {view.canRemove ? (
            <Button type="button" size="xs" variant="ghost" loading={pending === 'remove'} onClick={() => request('remove', view.removeWarning)}>
              <Trash2 aria-hidden />
              Quitar
            </Button>
          ) : null}
        </span>
      </div>
      {first !== undefined ? (
        <MediaPlayer items={items} activeId={first.id} open={playerOpen} onOpenChange={setPlayerOpen} returnFocus={returnFocus} />
      ) : null}
      {/* Stays mounted while open so the dialog can say the link worked. */}
      {view.needsLink || linkOpen ? <CloudLinkDialog open={linkOpen} onOpenChange={setLinkOpen} /> : null}
      {confirm !== null ? (
        <Dialog
          open={confirmOpen}
          onOpenChange={(open) => {
            if (pending === null) setConfirmOpen(open);
          }}
        >
          <DialogContent className="max-w-sm">
            <DialogHeader>
              <DialogTitle className="uppercase">{CONFIRM_COPY[confirm.action].title}</DialogTitle>
              <DialogDescription className="break-words">
                <span className="font-medium text-fg-1">{job.title}</span>. {confirm.warning}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" disabled={pending !== null} onClick={() => setConfirmOpen(false)}>
                {CONFIRM_COPY[confirm.action].keep}
              </Button>
              <Button
                type="button"
                variant="destructive"
                loading={pending !== null}
                onClick={confirm.action === 'cancel' ? cancel : remove}
              >
                {CONFIRM_COPY[confirm.action].confirm}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      ) : null}
    </>
  );
}

/** The hub rebuilds its lists on every poll, so props compare by value, not identity. */
export const CloudOutputItem = memo(CloudOutputItemCard, sameHubProps);

/** Saves a cloud video through the browser download, then offers the folder once the desktop saw it land. */
function SaveVideo({ jobId, name, title, labelled }: { jobId: string; name: string; title: string; labelled: boolean }): ReactNode {
  const url = cloudApi.videoUrl(jobId, name);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let active = true;
    setSaved(false);
    const bridge = downloadsBridge();
    if (bridge === null) return;
    const refresh = (): void => {
      void bridge
        .isSaved(absolute(url))
        .then((value) => {
          if (active) setSaved(value);
        })
        .catch(() => {});
    };
    refresh();
    const timer = setInterval(refresh, SAVED_POLL_MS);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [url]);

  return (
    <>
      <Button asChild size="xs" variant="outline-primary">
        <a href={url} download={streamVideoFilename(labelled ? `${title} ${name.replace(/\.mp4$/, '')}` : title)} aria-label={`Guardar vídeo: ${title}`}>
          <Download aria-hidden />
          {labelled ? `Guardar ${name}` : 'Guardar'}
        </a>
      </Button>
      {saved ? (
        <Button
          type="button"
          size="xs"
          variant="outline"
          onClick={() => {
            void downloadsBridge()?.reveal(absolute(url));
          }}
        >
          <FolderOpen aria-hidden />
          Abrir carpeta
        </Button>
      ) : null}
    </>
  );
}
