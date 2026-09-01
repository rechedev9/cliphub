import { jobToMatch } from './api/jobs-index.ts';
import type { Match, RosterMatch } from './api/types.ts';

export const DEMO_CHEATER_SINGLE_FILE_HINT =
  'CheaterDetect analiza una partida. Suelta un solo .dem.';

export type PickedDemo = { ok: true; file: File } | { ok: false; error: string };

/** CheaterDetect screens one demo; a series drop belongs on Shorts. */
export function pickCheaterDetectDemo(files: readonly File[]): PickedDemo {
  const file = files[0];
  if (files.length !== 1 || !file) {
    return { ok: false, error: DEMO_CHEATER_SINGLE_FILE_HINT };
  }
  return { ok: true, file };
}

export type ScanMatchInput = {
  jobId: string;
  fileName: string;
  roster?: RosterMatch;
  createdAt?: string;
};

/** Roster-ready Demos row from a finished scan, without a POV parse. */
export function matchFromScan(input: ScanMatchInput): Match {
  return jobToMatch(
    {
      jobId: input.jobId,
      status: 'scanned',
      fileName: input.fileName,
      createdAt: input.createdAt ?? new Date().toISOString(),
    },
    input.roster ? { map: input.roster.map } : undefined,
  );
}
