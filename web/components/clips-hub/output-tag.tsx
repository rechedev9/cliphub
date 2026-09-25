import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import { OUTPUT_STATE, OUTPUT_TONE, outputTagLabel, type MatchOutput } from '@/lib/clips/hub';
import { StatusTag } from '@/components/studio/status-tag';

export type OutputTagProps = {
  output: Pick<MatchOutput, 'state' | 'percent' | 'rounds'>;
  className?: string;
};

/** State tag with its motion cue: check, spinner, REC pulse, hollow queue dot. */
export function OutputTag({ output, className }: OutputTagProps): ReactNode {
  const label = outputTagLabel(output);
  const tone = OUTPUT_TONE[output.state];
  switch (output.state) {
    case OUTPUT_STATE.ready:
      return (
        <StatusTag tone={tone} icon={Check} className={className}>
          {label}
        </StatusTag>
      );
    case OUTPUT_STATE.render:
      return (
        <StatusTag tone={tone} className={className}>
          <span aria-hidden className="studio-spinner" />
          {label}
        </StatusTag>
      );
    case OUTPUT_STATE.rec:
      return (
        <StatusTag tone={tone} className={className}>
          <span aria-hidden className="neon-pulse size-1.5 shrink-0 rounded-full bg-current shadow-[0_0_6px_currentColor]" />
          {label}
        </StatusTag>
      );
    case OUTPUT_STATE.queue:
      return (
        <StatusTag tone={tone} className={className}>
          <span aria-hidden className="size-1.5 shrink-0 rounded-full border border-current" />
          {label}
        </StatusTag>
      );
    case OUTPUT_STATE.failed:
      return (
        <StatusTag tone={tone} className={className}>
          {label}
        </StatusTag>
      );
  }
}
