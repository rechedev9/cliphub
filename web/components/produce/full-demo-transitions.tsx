'use client';

import { useId, type ReactNode } from 'react';
import { withFullDemoHook, type FullDemoOptions } from '@/lib/full-demo-plan';
import { fullDemoTransitionPreset } from '@/lib/full-demo-transitions';
import { FULL_DEMO_HOOK_LABEL, FULL_DEMO_HOOK_NOTE, FULL_DEMO_TRANSITIONS_LABEL } from '@/lib/produce/full-demo-brief';
import { FullDemoFold, FullDemoToggle } from './full-demo-fields';

export function FullDemoTransitions({ options, onChange }: { options: FullDemoOptions; onChange: (value: FullDemoOptions) => void }): ReactNode {
  const enabled = options.transitions?.enabled ?? true;
  const hook = options.hook?.enabled === true;
  const hookNoteId = useId();
  return <FullDemoFold title="Entre rondas" value={`${enabled ? 'Con efectos' : 'Sin efectos'}${hook ? ', con gancho inicial' : ''}`} note="Transición dinámica entre rondas y gancho de apertura.">
    <FullDemoToggle label={FULL_DEMO_TRANSITIONS_LABEL} value={enabled} onChange={(next) => onChange({ ...options, transitions: { ...fullDemoTransitionPreset(), enabled: next } })} />
    <div className="grid min-w-0">
      <FullDemoToggle label={FULL_DEMO_HOOK_LABEL} value={hook} describedBy={hookNoteId} onChange={(next) => onChange(withFullDemoHook(options, next))} />
      <p id={hookNoteId} className="pl-7 text-meta text-fg-3">{FULL_DEMO_HOOK_NOTE}</p>
    </div>
  </FullDemoFold>;
}
