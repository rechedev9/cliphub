import type { ReactNode } from 'react';
import type { ProduceFormat } from '@/lib/clips/routes';
import { FLOW_HOME_DESCRIPTION, FLOW_HOME_TITLE } from '@/lib/produce/copy';
import { cn } from '@/lib/utils';
import { CreateFlowSteps } from '@/components/studio/create-flow-steps';
import { FORMAT_TONE } from '@/components/studio/format-tone';

/** The home band of a demo flow, tinted in its format colour. The sidebar switches formats, so the band names only its own. */
export function FlowHomeHero({ format }: { format: ProduceFormat }): ReactNode {
  const tone = FORMAT_TONE[format];
  return (
    <section aria-labelledby="flow-home-title" className={cn('flex flex-col gap-3.5 rounded-[20px] border p-5 @[48rem]/content:p-7', tone.tint, tone.border)}>
      <h1 id="flow-home-title" className="text-balance font-display text-display font-extrabold leading-[1.05] tracking-[-0.025em] text-fg-1 @[48rem]/content:text-[2.5rem]">
        {FLOW_HOME_TITLE[format]}
      </h1>
      <p className="max-w-xl text-body text-fg-2">{FLOW_HOME_DESCRIPTION[format]}</p>
      <CreateFlowSteps flow={format} step={0} className="mt-1" />
    </section>
  );
}
