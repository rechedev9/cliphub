'use client';

import { useId, type ReactNode } from 'react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { FORMAT_TONE } from '@/components/studio/format-tone';

/** A numbered settings panel; the number is the step's place in the editor, in the Demo completa colour. */
export function FullDemoGroup({ step, title, note, children }: { step: number; title: string; note?: string; children: ReactNode }): ReactNode {
  const headingId = useId();
  return <section aria-labelledby={headingId} className="studio-panel flex min-w-0 flex-col gap-3.5 rounded-2xl p-4">
    <div>
      <h2 id={headingId} className="flex items-center gap-2.5 font-display text-body-lg font-bold text-fg-1">
        <span aria-hidden className={`grid size-6 shrink-0 place-items-center rounded-lg text-meta font-extrabold text-format-full-foreground ${FORMAT_TONE.full.bg}`}>{step}</span>
        {title}
      </h2>
      {note ? <p className="mt-1.5 text-body-sm text-fg-2">{note}</p> : null}
    </div>
    {children}
  </section>;
}

/** A titled part inside a panel, for settings that share one step. */
export function FullDemoPart({ title, note, children }: { title: string; note?: string; children: ReactNode }): ReactNode {
  return <div className="min-w-0 space-y-2 border-t border-border-subtle pt-3.5 first:border-t-0 first:pt-0">
    <div><h3 className="text-body-sm font-semibold text-fg-1">{title}</h3>
      {note ? <p className="mt-1 text-body-sm text-fg-2">{note}</p> : null}</div>
    {children}
  </div>;
}

export function FullDemoToggle({ label, value, onChange }: { label: string; value: boolean; onChange: (value: boolean) => void }): ReactNode {
  return <label className="flex min-h-10 cursor-pointer items-center gap-3 text-body-sm text-fg-1">
    <input type="checkbox" className="size-4 accent-format-full" checked={value} onChange={(event) => onChange(event.target.checked)} />{label}
  </label>;
}

const SLIDER_CLASS =
  'h-1 w-full min-w-0 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-format-full disabled:cursor-not-allowed disabled:opacity-50';

/** Gain slider shown as a percentage: 1.0 gain renders as 100 %. */
export function FullDemoGain({ label, value, max = 2, step = 0.05, disabled = false, onChange }: {
  label: string; value: number; max?: number; step?: number; disabled?: boolean; onChange: (value: number) => void;
}): ReactNode {
  const id = useId();
  const percent = Math.round(value * 100);
  return <div className="flex min-w-0 items-center gap-3">
    <label htmlFor={id} className="w-24 shrink-0 text-meta text-fg-2">
      {label} <span className="text-fg-1">· {percent}%</span>
    </label>
    <input id={id} type="range" min={0} max={Math.round(max * 100)} step={Math.round(step * 100)} value={percent} disabled={disabled}
      aria-valuetext={`${percent}%`} className={SLIDER_CLASS} onChange={(event) => onChange(Number(event.target.value) / 100)} />
  </div>;
}

export function FullDemoChoice<T extends string>({ label, value, options, onChange }: {
  label: string; value: T; options: readonly { value: T; label: string }[]; onChange: (value: T) => void;
}): ReactNode {
  const id = useId();
  return <div className="space-y-1.5"><label className="text-body-sm text-fg-2" htmlFor={id}>{label}</label>
    <Select value={value} onValueChange={(next) => { const item = options.find((entry) => entry.value === next); if (item) onChange(item.value); }}>
    <SelectTrigger id={id} className="h-10 w-full"><SelectValue /></SelectTrigger>
      <SelectContent>{options.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
    </Select>
  </div>;
}
