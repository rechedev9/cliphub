'use client';

import { useEffect, useMemo, useState } from 'react';
import { autoShortsFiles } from '@/lib/api/auto-shorts';
import { dataPlane } from '@/lib/api/dataplane';
import { apiFetch } from '@/lib/api/edition-fetch';
import { DEFAULT_VARIANT } from '@/lib/api/reel-store';
import { OUTPUT_STATE, type HubModel } from '@/lib/clips/hub';
import type { LoteShort } from '@/lib/clips/visor-actions';

type Lote = { id: string; key: string; jobId: string; variant: string; artifactPrefix?: string };
type ReadLote = { key: string; shorts: readonly LoteShort[] };

function readyLotes(clips: HubModel['clips']): Lote[] {
  return clips.flatMap((clip) => {
    const { video } = clip;
    if (clip.state !== OUTPUT_STATE.ready || video.editConfig?.separateShorts !== true || video.jobId === undefined) return [];
    const key = [video.id, video.title, video.downloadUrl ?? '', video.reviewArtifactPrefix ?? ''].join('\n');
    return [{ id: video.id, key, jobId: video.jobId, variant: video.variant ?? DEFAULT_VARIANT, artifactPrefix: video.reviewArtifactPrefix }];
  });
}

async function readLote(lote: Lote, signal: AbortSignal): Promise<ReadLote> {
  const response = await apiFetch(dataPlane().renderUrl(lote.jobId, lote.variant), { cache: 'no-store', signal });
  const shorts = response.ok ? autoShortsFiles(await response.json(), lote).files : [];
  return { key: lote.key, shorts };
}

export function useLoteShorts(clips: HubModel['clips'] | null): ReadonlyMap<string, readonly LoteShort[]> {
  const [read, setRead] = useState<ReadonlyMap<string, ReadLote>>(() => new Map());
  const wanted = useMemo(
    () => (clips === null ? [] : readyLotes(clips).filter((lote) => read.get(lote.id)?.key !== lote.key)),
    [clips, read],
  );

  useEffect(() => {
    if (wanted.length === 0) return;
    const controller = new AbortController();
    void Promise.all(wanted.map((lote) => readLote(lote, controller.signal).catch(() => ({ key: lote.key, shorts: [] }))))
      .then((results) => {
        if (controller.signal.aborted) return;
        setRead((was) => {
          const next = new Map(was);
          wanted.forEach((lote, at) => {
            const result = results[at];
            if (result !== undefined) next.set(lote.id, result);
          });
          return next;
        });
      });
    return () => controller.abort();
  }, [wanted]);

  return useMemo(() => new Map(Array.from(read, ([id, lote]) => [id, lote.shorts])), [read]);
}
