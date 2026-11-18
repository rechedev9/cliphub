'use client';

import { useEffect, useState } from 'react';
import { fetchHorizontalDraft } from '@/lib/api/video-edit';
import type { HorizontalDraft } from '@/lib/produce/horizontal';

export type HorizontalDraftState = { status: 'idle' | 'loading' | 'failed'; draft: null } | { status: 'ready'; draft: HorizontalDraft };

/** Crear horizontal in Mis vídeos: the summary of one Short's plays, its capture and its music. */
export function useHorizontalDraft(jobId: string, variant: string | null): HorizontalDraftState {
  const [state, setState] = useState<HorizontalDraftState>({ status: variant === null ? 'idle' : 'loading', draft: null });

  useEffect(() => {
    if (variant === null) {
      setState({ status: 'idle', draft: null });
      return;
    }
    const controller = new AbortController();
    setState({ status: 'loading', draft: null });
    void fetchHorizontalDraft({ jobId, variant, signal: controller.signal })
      .then((draft) => {
        if (!controller.signal.aborted) setState(draft ? { status: 'ready', draft } : { status: 'failed', draft: null });
      })
      .catch(() => {
        if (!controller.signal.aborted) setState({ status: 'failed', draft: null });
      });
    return () => controller.abort();
  }, [jobId, variant]);

  return state;
}
