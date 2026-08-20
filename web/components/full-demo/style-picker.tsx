'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { api } from '@/lib/api';
import type { Preset } from '@/lib/api/types';
import { FULL_DEMO_PRESET, FULL_DEMO_VARIANT, resolveFullDemoPreset } from '@/lib/full-demo';
import { PresetCards } from '@/components/clips/preset-cards';

export function FullDemoStylePicker({
  value,
  onChange,
  disabled = false,
}: {
  value: string | null;
  onChange?: (variant: string) => void;
  disabled?: boolean;
}): ReactNode {
  const [presets, setPresets] = useState<Preset[]>([FULL_DEMO_PRESET]);

  useEffect(() => {
    let active = true;
    void api.listPresets().then((list) => {
      if (!active) return;
      setPresets([resolveFullDemoPreset(list)]);
    }).catch(() => {
      if (active) setPresets([FULL_DEMO_PRESET]);
    });
    return () => {
      active = false;
    };
  }, []);

  return (
    <section className="flex flex-col gap-3" aria-labelledby="full-demo-style-title">
      <h2 id="full-demo-style-title" className="font-mono text-meta uppercase tracking-wider text-fg-3">
        Estilo
      </h2>
      <PresetCards
        presets={presets}
        value={value}
        onChange={(name) => {
          if (name === FULL_DEMO_VARIANT) onChange?.(name);
        }}
        disabled={disabled}
      />
    </section>
  );
}
