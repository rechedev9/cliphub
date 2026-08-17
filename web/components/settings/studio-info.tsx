'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { MonitorCog } from 'lucide-react';
import { getDesktopSettingsBridge, type StudioAppInfo } from '@/lib/desktop-settings';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { StudioDataRow } from '@/components/studio/data-row';
import { Skeleton } from '@/components/ui/skeleton';

/** The build facts, in the order an operator reads them. */
const INFO_ROWS: ReadonlyArray<{ key: keyof StudioAppInfo; label: string }> = [
  { key: 'version', label: 'Versión' },
  { key: 'build', label: 'Build' },
  { key: 'electronVersion', label: 'Electron' },
  { key: 'chromiumVersion', label: 'Chromium' },
];

/**
 * The installed-build spec sheet. Version strings are operational metadata, so
 * they are typeset in the mono face with tabular figures through the shared
 * data row rather than as `text-sm` body copy in a `<dl>`.
 */
export function StudioInfo(): ReactNode {
  const [info, setInfo] = useState<StudioAppInfo | null>(null);
  const [unavailable, setUnavailable] = useState(false);

  useEffect(() => {
    const bridge = getDesktopSettingsBridge();
    if (!bridge) {
      setUnavailable(true);
      return;
    }
    void bridge.getAppInfo().then(setInfo).catch(() => setUnavailable(true));
  }, []);

  let body: ReactNode;
  if (info) {
    body = (
      <div className="flex flex-col gap-2">
        {INFO_ROWS.map((row) => (
          <StudioDataRow key={row.key} label={row.label} value={info[row.key]} />
        ))}
      </div>
    );
  } else if (unavailable) {
    body = (
      <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
        La versión instalada solo está disponible dentro de la app de escritorio.
      </p>
    );
  } else {
    body = (
      <div role="status" aria-label="Leyendo versión instalada" className="flex flex-col gap-2">
        {INFO_ROWS.map((row) => (
          <Skeleton key={row.key} className="h-11 w-full rounded-none" />
        ))}
      </div>
    );
  }

  return (
    <section className="studio-panel flex flex-col gap-5 p-5 sm:p-6" aria-labelledby="studio-info-title">
      <div className="flex items-center gap-4">
        <IconTile icon={MonitorCog} size="md" depth="inset" />
        <div className="flex min-w-0 flex-col gap-1">
          <SectionEyebrow label="APLICACIÓN" />
          <h2 id="studio-info-title" className="font-display text-title font-bold uppercase text-fg-1">
            ClipHub Studio
          </h2>
        </div>
      </div>

      {body}
    </section>
  );
}
