import type { MediaPlaybackItem } from '../api/playback.ts';
import type { Video } from '../api/types.ts';
import { PUBLISH_PREP_LABEL } from '../publish-actions.ts';
import type { HubModel } from './hub.ts';
import { ORPHAN_MATCH_SEGMENT, PRODUCE_FORMAT, editVideoHref, loteShortPublishHref, publishHref, streamEditorHref } from './routes.ts';

export const VISOR_ACTION = {
  edit: 'edit',
  music: 'music',
  publish: 'publish',
  openProject: 'open-project',
  download: 'download',
  delete: 'delete',
} as const;

export type VisorAction =
  | { kind: typeof VISOR_ACTION.edit; label: string; href: string }
  | { kind: typeof VISOR_ACTION.music; label: string }
  | { kind: typeof VISOR_ACTION.publish; label: string; href: string }
  | { kind: typeof VISOR_ACTION.openProject; label: string; href: string }
  | { kind: typeof VISOR_ACTION.download; label: string }
  | { kind: typeof VISOR_ACTION.delete; label: string };

export type LoteShort = { name: string; url: string };

export type VisorSource =
  | { kind: 'demo'; video: Video; loteShort?: LoteShort }
  | { kind: 'stream'; clipId: string };

const DOWNLOAD: VisorAction = { kind: VISOR_ACTION.download, label: 'Descargar' };
const DELETE: VisorAction = { kind: VISOR_ACTION.delete, label: 'Borrar' };

function musicLabel(video: Video, lote: boolean): string {
  if (!lote) return video.songId ? 'Cambiar música' : 'Añadir música';
  return video.songId ? 'Cambiar música del lote' : 'Añadir música al lote';
}

export function demoShortActions(clip: HubModel['clips'][number], loteShort?: LoteShort): VisorAction[] {
  const { video } = clip;
  const matchId = clip.match?.id ?? null;
  const editable = video.recovered !== true;
  const lote = loteShort !== undefined;
  const actions: VisorAction[] = [];
  if (editable && matchId !== null && video.variant !== undefined) {
    const href = editVideoHref({ matchId, format: PRODUCE_FORMAT.short, variant: video.variant });
    actions.push({ kind: VISOR_ACTION.edit, label: lote ? 'Editar el lote' : 'Editar', href });
  }
  if (editable) actions.push({ kind: VISOR_ACTION.music, label: musicLabel(video, lote) });
  if (video.publishable !== false) {
    const publishMatch = matchId ?? ORPHAN_MATCH_SEGMENT;
    const href = loteShort === undefined
      ? publishHref(publishMatch, video.id)
      : loteShortPublishHref({ matchId: publishMatch, clipId: video.id, short: loteShort.name });
    actions.push({ kind: VISOR_ACTION.publish, label: PUBLISH_PREP_LABEL, href });
  }
  return [...actions, DOWNLOAD, DELETE];
}

export function streamShortActions(item: MediaPlaybackItem): VisorAction[] {
  return [{ kind: VISOR_ACTION.openProject, label: 'Abrir proyecto', href: streamEditorHref(item.jobId) }, DOWNLOAD, DELETE];
}
