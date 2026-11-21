'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { autoShortsFiles, isLote } from '@/lib/api/auto-shorts';
import { dataPlane } from '@/lib/api/dataplane';
import { apiFetch } from '@/lib/api/edition-fetch';
import { DEFAULT_VARIANT } from '@/lib/api/reel-store';
import { OUTPUT_STATE, type HubModel } from '@/lib/clips/hub';
import type { LoteShort } from '@/lib/clips/visor-actions';

type Lote = { id: string; key: string; jobId: string; variant: string; artifactPrefix?: string };
type ReadLote = { key: string; shorts: readonly LoteShort[] };

export type LoteShortsRead = { lotes: ReadonlyMap<string, readonly LoteShort[]>; retry: () => void };

const NONE_FAILED: ReadonlySet<string> = new Set();

function readyLotes(clips: HubModel['clips']): Lote[] {
  return clips.flatMap((clip) => {
    const { video } = clip;
    if (clip.state !== OUTPUT_STATE.ready || !isLote(video) || video.jobId === undefined) return [];
    const key = [video.id, video.title, video.downloadUrl ?? '', video.reviewArtifactPrefix ?? ''].join('\n');
    return [{ id: video.id, key, jobId: video.jobId, variant: video.variant ?? DEFAULT_VARIANT, artifactPrefix: video.reviewArtifactPrefix }];
  });
}

async function readLote(lote: Lote, signal: AbortSignal): Promise<ReadLote> {
  const response = await apiFetch(dataPlane().renderUrl(lote.jobId, lote.variant), { cache: 'no-store', signal });
  if (response.status === 404) return { key: lote.key, shorts: [] };
  if (!response.ok) throw new Error(`render ${response.status}`);
  return { key: lote.key, shorts: autoShortsFiles(await response.json(), lote).files };
}

export function useLoteShorts(clips: HubModel['clips'] | null): LoteShortsRead {
  const [read, setRead] = useState<ReadonlyMap<string, ReadLote>>(() => new Map());
  const [failed, setFailed] = useState<ReadonlySet<string>>(NONE_FAILED);
  const wanted = useMemo(
    () => (clips === null ? [] : readyLotes(clips).filter((lote) => read.get(lote.id)?.key !== lote.key && !failed.has(lote.key))),
    [clips, read, failed],
  );

  useEffect(() => {
    if (wanted.length === 0) return;
    const controller = new AbortController();
    void Promise.allSettled(wanted.map((lote) => readLote(lote, controller.signal))).then((results) => {
      if (controller.signal.aborted) return;
      const done = wanted.flatMap((lote, at) => {
        const result = results[at];
        return result?.status === 'fulfilled' ? [{ id: lote.id, value: result.value }] : [];
      });
      const lost = wanted.filter((_, at) => results[at]?.status !== 'fulfilled').map((lote) => lote.key);
      if (lost.length > 0) setFailed((was) => new Set([...was, ...lost]));
      if (done.length > 0) setRead((was) => new Map([...was, ...done.map(({ id, value }): [string, ReadLote] => [id, value])]));
    });
    return () => controller.abort();
  }, [wanted]);

  const retry = useCallback(() => setFailed((was) => (was.size === 0 ? was : NONE_FAILED)), []);
  const lotes = useMemo(() => new Map(Array.from(read, ([id, lote]) => [id, lote.shorts])), [read]);
  return { lotes, retry };
}
