'use client';

import { useId, type ReactNode } from 'react';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

export function FullDemoGroup({ title, note, children }: { title: string; note?: string; children: ReactNode }): ReactNode {
  return <section className="studio-panel space-y-4 p-4">
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

export function FullDemoChoice<T extends string>({ label, value, options, onChange }: {
  label: string; value: T; options: readonly { value: T; label: string }[]; onChange: (value: T) => void;
}): ReactNode {
  const id = useId();
  return <div className="space-y-1.5"><label className="text-body-sm text-fg-2" htmlFor={id}>{label}</label>
    <Select value={value} onValueChange={(next) => { const item = options.find((entry) => entry.value === next); if (item) onChange(item.value); }}>
      <SelectTrigger id={id} className="w-full"><SelectValue /></SelectTrigger>
      <SelectContent>{options.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
    </Select>
  </div>;
}
