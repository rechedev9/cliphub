'use client';

import { useId, type ReactNode } from 'react';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

export function FullDemoGroup({ title, note, children }: { title: string; note?: string; children: ReactNode }): ReactNode {
  return <section className="studio-panel min-w-0 space-y-2 p-3">
    <div><h2 className="font-display text-body-lg font-semibold uppercase text-fg-1">{title}</h2>
      {note ? <p className="mt-1 text-body-sm text-fg-2">{note}</p> : null}</div>
    {children}
  </section>;
}

export function FullDemoToggle({ label, value, onChange }: { label: string; value: boolean; onChange: (value: boolean) => void }): ReactNode {
  return <label className="flex min-h-10 cursor-pointer items-center gap-3 text-body-sm text-fg-1">
    <input type="checkbox" className="size-4 accent-primary" checked={value} onChange={(event) => onChange(event.target.checked)} />{label}
  </label>;
}

export function FullDemoNumber({ label, value, min = 0, max, step = 1, onChange }: {
  label: string; value: number; min?: number; max?: number; step?: number; onChange: (value: number) => void;
}): ReactNode {
  const id = useId();
  return <div className="space-y-1.5"><label className="text-body-sm text-fg-2" htmlFor={id}>{label}</label>
    <Input id={id} type="number" min={min} max={max} step={step} value={value} className="tabular-nums" onChange={(event) => {
      const next = event.target.valueAsNumber;
      if (Number.isFinite(next)) onChange(next);
    }} /></div>;
}

const SLIDER_CLASS =
  'h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-stream disabled:cursor-not-allowed disabled:opacity-50';

/** Gain slider shown as a percentage: 1.0 gain renders as 100 %. */
export function FullDemoGain({ label, value, max = 2, step = 0.05, disabled = false, onChange }: {
  label: string; value: number; max?: number; step?: number; disabled?: boolean; onChange: (value: number) => void;
}): ReactNode {
  const id = useId();
  const percent = Math.round(value * 100);
  return <div className="flex items-center gap-3">
    <label htmlFor={id} className="w-24 shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2">
      {label} <span className="text-stream-text">· {percent}%</span>
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
