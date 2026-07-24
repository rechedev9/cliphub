import { Badge } from '@/components/ui/badge';
import { ANTICHEAT_VERDICT, VERDICT_LABEL, type AnticheatVerdict } from '@/lib/api/anticheat';
import { cn } from '@/lib/utils';

/**
 * Per-band colour. Only the two reviewable bands carry alarm colour; the
 * inconclusive band stays amber and the clean and thin-sample bands stay
 * neutral, so a scoreboard never reads as an accusation at a glance.
 */
const VERDICT_CLASS: Record<AnticheatVerdict, string> = {
  [ANTICHEAT_VERDICT.highlyAnomalous]: 'border-destructive/50 bg-destructive/15 text-destructive',
  [ANTICHEAT_VERDICT.anomalous]: 'border-stream/50 bg-stream/15 text-stream',
  [ANTICHEAT_VERDICT.inconclusive]: 'border-amber-500/45 bg-amber-500/12 text-amber-500',
  [ANTICHEAT_VERDICT.clean]: 'border-primary/35 bg-primary/10 text-primary',
  [ANTICHEAT_VERDICT.insufficient]: 'border-border/80 bg-muted/40 text-muted-foreground',
};

export function VerdictBadge({ verdict, className }: { verdict: AnticheatVerdict; className?: string }) {
  return (
    <Badge
      variant="outline"
      className={cn(
        'font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.1em]',
        VERDICT_CLASS[verdict],
        className,
      )}
    >
      {VERDICT_LABEL[verdict]}
    </Badge>
  );
}
