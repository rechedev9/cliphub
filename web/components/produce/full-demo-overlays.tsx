'use client';

import { useId, type ReactNode } from 'react';
import { withFullDemoStoryOverlay, type FullDemoOptions } from '@/lib/full-demo-plan';
import { introDesignGallery, introDesignPreviewUrl } from '@/lib/overlay-design-gallery';
import { FULL_DEMO_HALFTIME_LABEL, FULL_DEMO_HALFTIME_NOTE, FULL_DEMO_SOURCE_LABEL, FULL_DEMO_SOURCES, FULL_DEMO_SUBSCRIBE_LABEL, FULL_DEMO_SUBSCRIBE_NOTE } from '@/lib/produce/full-demo-brief';
import { brandSubscribeOf } from '@/lib/brand-kit';
import { useBrandKit } from '@/components/clips/brand-kit-editor';
import { CoverImage } from '@/components/studio/cover-image';
import { DesignGallery } from '@/components/studio/design-gallery';
import { FullDemoChoice, FullDemoToggle } from './full-demo-fields';
import { PreviewBackdrop } from './full-demo-hud';

function StoryOverlayToggle({ label, note, value, onChange }: { label: string; note: string; value: boolean; onChange: (value: boolean) => void }): ReactNode {
  const noteId = useId();
  return <div className="grid min-w-0">
    <FullDemoToggle label={label} value={value} describedBy={noteId} onChange={onChange} />
    <p id={noteId} className="pl-7 text-meta text-fg-3">{note}</p>
  </div>;
}

export function FullDemoOverlays({ options, map, onChange, onAssetBusy }: { options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void }): ReactNode {
  void map; void onAssetBusy;
  const { kit } = useBrandKit();
  const card = kit ? brandSubscribeOf(kit) : undefined;
  const subscribeNote = options.overlays.brand?.subscribe || (options.overlays.subscribe !== true && card)
    ? `${FULL_DEMO_SUBSCRIBE_NOTE} Con tu logo y el nombre del canal del kit de marca.`
    : FULL_DEMO_SUBSCRIBE_NOTE;
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
    <StoryOverlayToggle label={FULL_DEMO_HALFTIME_LABEL} note={FULL_DEMO_HALFTIME_NOTE} value={options.overlays.halftime === true}
      onChange={(enabled) => onChange(withFullDemoStoryOverlay(options, 'halftime', enabled))} />
    <StoryOverlayToggle label={FULL_DEMO_SUBSCRIBE_LABEL} note={subscribeNote} value={options.overlays.subscribe === true}
      onChange={(enabled) => onChange(withFullDemoStoryOverlay(options, 'subscribe', enabled, card))} />
  </div>;
}
