'use client';

import { useEffect, useLayoutEffect, useRef } from 'react';
import type { HubModel } from '@/lib/clips/hub';
import { isHubLens, type HubLens } from '@/lib/clips/routes';
import { previewModelContext, registerWebTools, waitForWebState } from '@/lib/webmcp';

type State = {
  model: HubModel | null;
  lens: HubLens;
  open: string | null;
  failed: boolean;
  navigate: (next: { lens?: HubLens; open?: string }) => void;
};

export function useWebMCPLibrary(state: State): void {
  const current = useRef(state);
  useLayoutEffect(() => { current.current = state; });
  useEffect(() => {
    const snapshot = () => {
      const { model, lens, open, failed } = current.current;
      let dataStatus = model === null ? 'loading' : 'ready';
      if (failed) dataStatus = 'unavailable_or_partial';
      return {
        view: lens, open_match_id: open,
        data_status: dataStatus,
        matches: model?.rows.slice(0, 100).map(row => ({ id: row.match.id, map: row.match.map, stage: row.stage })) ?? [],
        clips: model?.clips.slice(0, 100).map(clip => ({ id: clip.id, title: clip.title, state: clip.state })) ?? [],
        truncated: (model?.rows.length ?? 0) > 100 || (model?.clips.length ?? 0) > 100,
      };
    };
    return registerWebTools(previewModelContext(), [
      {
        name: 'cliphub_get_library',
        description: 'Read the current demo library model, selected view and loading/failure status. Results can be partial when the local service is unavailable. Stream outputs are not included.',
        inputSchema: { type: 'object', properties: {}, additionalProperties: false },
        annotations: { readOnlyHint: true },
        execute: () => JSON.stringify(snapshot()),
      },
      {
        name: 'cliphub_set_library_view',
        description: 'Select the partidas or clips view using ClipHub application state.',
        inputSchema: { type: 'object', properties: { view: { type: 'string', enum: ['partidas', 'clips'] } }, required: ['view'], additionalProperties: false },
        execute: async ({ view }) => {
          if (typeof view !== 'string' || !isHubLens(view)) throw new Error('Unknown library view');
          current.current.navigate({ lens: view });
          await waitForWebState(() => current.current.lens === view && current.current.open === null);
          return JSON.stringify(snapshot());
        },
      },
      {
        name: 'cliphub_open_match',
        description: 'Expand an existing match in the library. Use an id from cliphub_get_library.',
        inputSchema: { type: 'object', properties: { match_id: { type: 'string' } }, required: ['match_id'], additionalProperties: false },
        execute: async ({ match_id }) => {
          if (typeof match_id !== 'string' || !current.current.model?.rows.some(row => row.match.id === match_id)) throw new Error('Match is not present in the current library');
          current.current.navigate({ open: match_id });
          await waitForWebState(() => current.current.open === match_id);
          return JSON.stringify(snapshot());
        },
      },
    ]);
  }, []);
}
