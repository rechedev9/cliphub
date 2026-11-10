import type { ReportRoundList } from '../api/scout.ts';
import { replayHref } from '../analysis/replay-link.ts';

/** The Replay playlist of a report's round list, the same path the MCP hands out as its `studio_path`. */
export function roundListHref(list: ReportRoundList): string {
  return replayHref(
    list.rounds.map((ref) => ({ matchId: ref.match_id, round: ref.round })),
    list.title,
  );
}
