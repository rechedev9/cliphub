'use client';

import { useEffect, useLayoutEffect, useRef } from 'react';
import type { HubModel } from '@/lib/clips/hub';
import type { ResultsOrigin } from '@/lib/clips/routes';
import { previewModelContext, registerWebTools } from '@/lib/webmcp';

type State = {
  model: HubModel | null;
  origin: ResultsOrigin | null;
  failed: boolean;
};

export function useWebMCPLibrary(state: State): void {
  const current = useRef(state);
  useLayoutEffect(() => { current.current = state; });
  useEffect(() => {
    const snapshot = () => {
      const { model, origin, failed } = current.current;
      let dataStatus = model === null ? 'loading' : 'ready';
      if (failed) dataStatus = 'unavailable_or_partial';
      return {
        origin_filter: origin ?? 'all',
        data_status: dataStatus,
        matches: model?.rows.slice(0, 100).map(row => ({ id: row.match.id, map: row.match.map, stage: row.stage })) ?? [],
        clips: model?.clips.slice(0, 100).map(clip => ({ id: clip.id, title: clip.title, state: clip.state })) ?? [],
        truncated: (model?.rows.length ?? 0) > 100 || (model?.clips.length ?? 0) > 100,
      };
    };
    return registerWebTools(previewModelContext(), [
      {
        name: 'cliphub_get_library',
        description: 'Read the loaded demos and the videos recorded on this PC, with the loading/failure status. Results can be partial when the local service is unavailable. Stream and cloud outputs are not included.',
        inputSchema: { type: 'object', properties: {}, additionalProperties: false },
        annotations: { readOnlyHint: true },
        execute: () => JSON.stringify(snapshot()),
      },
    ]);
  }, []);
}
