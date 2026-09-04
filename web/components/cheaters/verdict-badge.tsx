import type { ReactNode } from 'react';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { ANTICHEAT_VERDICT, VERDICT_LABEL, type AnticheatVerdict } from '@/lib/api/anticheat';

/**
 * Per-band tone. Only the two reviewable bands carry alarm colour, so a
 * scoreboard never reads as an accusation at a glance.
 */
const VERDICT_TONE: Record<AnticheatVerdict, StatusTagTone> = {
  [ANTICHEAT_VERDICT.highlyAnomalous]: 'danger',
  [ANTICHEAT_VERDICT.anomalous]: 'stream',
  [ANTICHEAT_VERDICT.inconclusive]: 'warning',
  [ANTICHEAT_VERDICT.clean]: 'primary',
  [ANTICHEAT_VERDICT.insufficient]: 'neutral',
};

export function VerdictBadge({ verdict, className }: { verdict: AnticheatVerdict; className?: string }): ReactNode {
  return (
    <StatusTag tone={VERDICT_TONE[verdict]} className={className}>
      {VERDICT_LABEL[verdict]}
    </StatusTag>
  );
}
