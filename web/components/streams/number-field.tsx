'use client';
import { useEffect, useState, type ReactNode } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
export function NumberField({
  id,
  label,
  value,
  max,
  invalid,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  max?: number;
  invalid?: boolean;
  disabled: boolean;
  onChange: (value: number) => void;
}): ReactNode {
  // Local text while focused, so clearing the field to retype does not
  // immediately commit 0 into the plan; commit only a valid number, on blur.
  const [text, setText] = useState(String(value));
  const [focused, setFocused] = useState(false);
  useEffect(() => {
    if (!focused) setText(String(value));
  }, [value, focused]);

  const commit = (): void => {
    const parsed = Number(text);
    if (text.trim() !== '' && Number.isFinite(parsed)) {
      onChange(parsed);
    } else {
      setText(String(value));
    }
  };

  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Label htmlFor={id} className="text-label text-fg-2">
        {label}
      </Label>
      <Input
        id={id}
        type="number"
        min={0}
        max={max}
        step="0.1"
        value={text}
        disabled={disabled}
        aria-invalid={invalid}
        onFocus={() => setFocused(true)}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.currentTarget.blur();
        }}
        onBlur={() => {
          setFocused(false);
          commit();
        }}
        className="h-9 tabular-nums"
      />
    </div>
  );
}
