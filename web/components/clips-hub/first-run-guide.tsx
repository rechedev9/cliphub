import type { ReactNode } from 'react';
import { Check, X } from 'lucide-react';
import { FIRST_RUN_GUIDE_DISMISS, FIRST_RUN_GUIDE_TITLE, FIRST_RUN_STEPS } from '@/lib/clips/copy';
import { FIRST_RUN_STEP, type FirstRunProgress, type FirstRunStep } from '@/lib/clips/hub';
import { cn } from '@/lib/utils';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';

export type FirstRunGuideProps = {
  progress: FirstRunProgress;
  /** Absent on the empty hub: with nothing loaded the guide is the explanation, not a banner. */
  onDismiss?: () => void;
};

const STEPS = Object.values(FIRST_RUN_STEP);

/** Three numbered steps that tick from real hub data; the first pending one is the active step. */
export function FirstRunGuide({ progress, onDismiss }: FirstRunGuideProps): ReactNode {
  const active = STEPS.find((step) => !progress[step]) ?? null;
  return (
    <section aria-label={FIRST_RUN_GUIDE_TITLE} className="studio-panel studio-enter flex flex-col gap-3 px-4 py-3.5">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-mono text-meta uppercase tracking-widest text-fg-3">{FIRST_RUN_GUIDE_TITLE}</h2>
        {onDismiss ? (
          <Button type="button" variant="ghost" size="icon-xs" aria-label={FIRST_RUN_GUIDE_DISMISS} onClick={onDismiss}>
            <X aria-hidden />
          </Button>
        ) : null}
      </div>
      <ol className="grid gap-3 sm:grid-cols-3">
        {STEPS.map((step, index) => (
          <GuideStep key={step} step={step} index={index} done={progress[step]} active={step === active} />
        ))}
      </ol>
    </section>
  );
}

function GuideStep({
  step,
  index,
  done,
  active,
}: {
  step: FirstRunStep;
  index: number;
  done: boolean;
  active: boolean;
}): ReactNode {
  const copy = FIRST_RUN_STEPS[step];
  let tone: 'success' | 'primary' | 'neutral' = 'neutral';
  let titleClass = 'text-fg-2';
  if (done) {
    tone = 'success';
    titleClass = 'text-fg-3 line-through';
  } else if (active) {
    tone = 'primary';
    titleClass = 'text-fg-1';
  }
  return (
    <li aria-current={active ? 'step' : undefined} className="flex items-start gap-3">
      <StatusTag tone={tone} className="min-w-7 justify-center tabular-nums">
        {done ? <Check aria-hidden className="size-3.5" /> : index + 1}
      </StatusTag>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className={cn('font-display text-label font-bold uppercase tracking-wide', titleClass)}>
          {copy.title}
        </span>
        <span className="text-body-sm text-fg-3">{copy.hint}</span>
      </span>
    </li>
  );
}
