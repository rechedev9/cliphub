'use client';

import { memo, type ReactNode } from 'react';
import Link from 'next/link';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import type { ItemStatus, QueueItem } from '@/lib/browser-import/import-queue';
import { matchHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { formatBytes } from '@/lib/storage';

const STATUS: Record<ItemStatus, { label: string; tone: StatusTagTone }> = {
  queued: { label: 'En cola', tone: 'neutral' },
  scanning: { label: 'Analizando', tone: 'primary' },
  scanned: { label: 'Esperando para subir', tone: 'neutral' },
  uploading: { label: 'Subiendo', tone: 'primary' },
  imported: { label: 'Importada', tone: 'success' },
  duplicate: { label: 'Ya estaba en la biblioteca', tone: 'neutral' },
  failed: { label: 'Error', tone: 'danger' },
};

const linkClass = 'text-body-sm text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1';

interface RowProps {
  item: QueueItem;
  onRetry: (id: string) => void;
  onRemove: (id: string) => void;
}

function statusLabel(item: QueueItem): string {
  const { label } = STATUS[item.status];
  return item.status === 'scanning' || item.status === 'uploading' ? `${label} ${item.percent}%` : label;
}

/** One demo of the queue. Memoised: a progress tick repaints only the row that moved. */
export const WebImportRow = memo(function WebImportRow({ item, onRetry, onRemove }: RowProps): ReactNode {
  const inFlight = item.status === 'scanning' || item.status === 'uploading';
  const removable = item.status === 'queued' || item.status === 'scanning' || item.status === 'failed';
  const meta = item.map === undefined ? formatBytes(item.size) : `${formatBytes(item.size)} · ${prettyMapName(item.map)}`;
  return (
    <li className="flex flex-col gap-2 px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <div className="min-w-0 flex-1 basis-56">
          <p className="truncate font-mono text-meta text-fg-1" title={item.name}>
            {item.name}
          </p>
          <p className="text-meta text-fg-3">{meta}</p>
        </div>
        <StatusTag tone={STATUS[item.status].tone} dot className="tabular-nums">
          {statusLabel(item)}
        </StatusTag>
        <div className="flex shrink-0 items-center gap-2">
          {item.matchId !== undefined && (item.status === 'imported' || item.status === 'duplicate') ? (
            <Link href={matchHref(item.matchId)} className={linkClass}>
              Abrir la partida
            </Link>
          ) : null}
          {item.status === 'failed' ? (
            <Button type="button" size="xs" variant="outline" aria-label={`Reintentar ${item.name}`} onClick={() => onRetry(item.id)}>
              Reintentar
            </Button>
          ) : null}
          {removable ? (
            <Button type="button" size="xs" variant="ghost" aria-label={`Quitar ${item.name} de la lista`} onClick={() => onRemove(item.id)}>
              Quitar
            </Button>
          ) : null}
        </div>
      </div>
      {inFlight ? <Progress size="xs" value={item.percent} aria-label={`${STATUS[item.status].label} ${item.name}`} /> : null}
      {item.status === 'failed' ? (
        <p className="text-body-sm text-destructive">
          {item.error}
          {item.detail === undefined ? null : <span className="block break-words font-mono text-meta text-fg-3">{item.detail}</span>}
        </p>
      ) : null}
    </li>
  );
});
