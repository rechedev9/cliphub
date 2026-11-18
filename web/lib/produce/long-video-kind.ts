import { FLOW_HREF, PRODUCE_FORMAT, PRODUCE_QUERY } from '../clips/routes.ts';

/** What a long video of the Vídeos largos flow films, by its `?video=` value. */
export const LONG_VIDEO_KIND = { match: 'partida', summary: 'resumen' } as const;
export type LongVideoKind = (typeof LONG_VIDEO_KIND)[keyof typeof LONG_VIDEO_KIND];

type ProduceQuery = Readonly<Record<string, string | string[] | undefined>>;

/** One player films the whole match or a summary of their kills; 2 to 5 players make a summary. */
export function longVideoKinds(povPlayers: number): LongVideoKind[] {
  return povPlayers > 1 ? [LONG_VIDEO_KIND.summary] : [LONG_VIDEO_KIND.match, LONG_VIDEO_KIND.summary];
}

/** The kind the URL asks for when the players allow it, else the first one offered. */
export function longVideoKind(povPlayers: number, requested: string | string[] | undefined): LongVideoKind {
  const kinds = longVideoKinds(povPlayers);
  return kinds.find((kind) => kind === requested) ?? kinds[0];
}

/** The producer of one job on another kind, keeping the rest of its query. */
export function longVideoKindHref({ jobId, kind, query }: { jobId: string; kind: LongVideoKind; query: ProduceQuery }): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (key !== PRODUCE_QUERY.video && typeof value === 'string') params.set(key, value);
  }
  if (kind === LONG_VIDEO_KIND.summary) params.set(PRODUCE_QUERY.video, kind);
  const base = `${FLOW_HREF[PRODUCE_FORMAT.full]}/${encodeURIComponent(jobId)}`;
  return params.size > 0 ? `${base}?${params.toString()}` : base;
}
