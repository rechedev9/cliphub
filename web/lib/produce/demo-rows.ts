import { HUB_ROW_STAGE, type HubMatch, type MatchOutput } from '../clips/hub.ts';

/** One demo of the "Demos cargadas" list: the job its row opens, and every job made from that demo. */
export type DemoRowGroup = HubMatch & { readonly jobIds: readonly string[] };

function byNewest(a: MatchOutput, b: MatchOutput): number {
  return b.video.createdAt - a.video.createdAt;
}

/** The armed delete text of a demo row, which removes every job of the demo and all their videos. */
export function demoDeletePrompt(group: DemoRowGroup): string {
  const videos = group.shorts.length + group.fulls.length;
  if (videos === 0) return '¿Borrar?';
  return videos === 1 ? '¿Borrar con 1 vídeo?' : `¿Borrar con ${videos} vídeos?`;
}

/** Folds the jobs of one demo (another POV, a job from Analizar) into one row that opens the
 * newest ready job, else the newest, with the videos of all of them. `rows` are newest first. */
export function oneRowPerDemo(rows: readonly HubMatch[]): DemoRowGroup[] {
  const groups = new Map<string, [HubMatch, ...HubMatch[]]>();
  for (const row of rows) {
    const key = row.match.demoSha256 ?? `job:${row.match.id}`;
    const group = groups.get(key);
    if (group === undefined) groups.set(key, [row]);
    else group.push(row);
  }
  return Array.from(groups.values(), (group) => {
    const shown = group.find((row) => row.stage === HUB_ROW_STAGE.ready) ?? group[0];
    return {
      ...shown,
      shorts: group.flatMap((row) => row.shorts).sort(byNewest),
      fulls: group.flatMap((row) => row.fulls).sort(byNewest),
      jobIds: group.map((row) => row.match.id),
    };
  });
}
