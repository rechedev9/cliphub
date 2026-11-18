'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { introDesignGallery, introDesignPreviewUrl } from '@/lib/overlay-design-gallery';
import { FULL_DEMO_SOURCE_LABEL, FULL_DEMO_SOURCES } from '@/lib/produce/full-demo-brief';
import { CoverImage } from '@/components/studio/cover-image';
import { DesignGallery } from '@/components/studio/design-gallery';
import { FullDemoChoice } from './full-demo-fields';
import { PreviewBackdrop } from './full-demo-hud';

export function FullDemoOverlays({ options, map, onChange, onAssetBusy }: { options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void }): ReactNode {
  void map; void onAssetBusy;
  const chosen = options.overlays.theme === 'hud' ? 'hud' : 'neon';
  function choose(theme: string): void {
    if (theme === 'hud' || theme === 'neon-violet') onChange({ ...options, overlays: { ...options.overlays, theme } });
  }
  return <div className="grid min-w-0 gap-3">
    <DesignGallery label="Diseño" items={introDesignGallery(options.overlays)} shape="video" tone="full" backdrop={<PreviewBackdrop />} onSelect={choose} />
    <figure className="grid min-w-0 max-w-64 gap-1.5">
      <span className="relative isolate block aspect-video w-full overflow-hidden rounded-md bg-surface-0">
        <PreviewBackdrop />
        <CoverImage src={introDesignPreviewUrl({ design: chosen, overlays: options.overlays, outro: true })} className="absolute inset-0 object-contain" />
      </span>
      <figcaption className="text-meta text-fg-3">El marcador final, con el mismo diseño.</figcaption>
    </figure>
    <FullDemoChoice label={FULL_DEMO_SOURCE_LABEL} value={options.source_kind} options={FULL_DEMO_SOURCES} onChange={(source_kind) => onChange({ ...options, source_kind })} />
  </div>;
}
