'use client';

import { useEffect, useState } from 'react';
import { listVideoVersions } from '@/lib/api/video-edit';
import type { VideoEdit } from '@/lib/clips/routes';
import { pickVersion, shortSettingsFromVersion, versionDateLabel, type ShortEditSeed } from '@/lib/produce/edit-capture';

export type ShortEditSeedState = { status: 'idle' | 'loading' | 'failed'; seed: null } | { status: 'ready'; seed: ShortEditSeed };

/** Editar in Mis vídeos: the plays, options and music of one Short or resumen version. */
export function useShortEditSeed(jobId: string, edit: VideoEdit | null): ShortEditSeedState {
  const variant = edit?.variant ?? null;
  const revision = edit?.revision ?? null;
  const [state, setState] = useState<ShortEditSeedState>({ status: variant === null ? 'idle' : 'loading', seed: null });

  useEffect(() => {
    if (variant === null) {
      setState({ status: 'idle', seed: null });
      return;
    }
    const controller = new AbortController();
    setState({ status: 'loading', seed: null });
    void listVideoVersions({ jobId, variant, signal: controller.signal })
      .then((versions) => {
        if (controller.signal.aborted) return;
        const version = pickVersion(versions, revision);
        const settings = version ? shortSettingsFromVersion({ ...version, variant }) : null;
        setState(version && settings ? { status: 'ready', seed: { settings, date: versionDateLabel(version.createdAt) } } : { status: 'failed', seed: null });
      })
      .catch(() => {
        if (!controller.signal.aborted) setState({ status: 'failed', seed: null });
      });
    return () => controller.abort();
  }, [jobId, variant, revision]);

  return state;
}
