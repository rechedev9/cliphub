'use client';

import type { ReactNode } from 'react';
import { STREAM_VARIANTS, type StreamEditPlan, type StreamJob, type StreamVariant } from '@/lib/api/streams';
import { isKeyDropBannerStyle, type Song } from '@/lib/api/types';
import { killFeedDefaultPosition } from '@/lib/streams/kill-feed';
import { STREAMER_NICK_RE } from '@/lib/streams/plan';
import { StreamBannerControls } from '@/components/streams/banner-controls';
import { isKeyDropCodeValid, StreamKeyDropBannerControls } from '@/components/streams/keydrop-banner-controls';
import { StreamKillFeedControls } from '@/components/streams/kill-feed-controls';
import { StreamMusicCard } from '@/components/streams/music-card';
import type { StreamBannerActions } from '@/components/streams/stream-banner-actions';
import { StreamLayoutBar } from '@/components/streams/stream-layout-bar';
import { StreamLayoutStep } from '@/components/streams/stream-step-panel';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';

export function StreamLookStep({
  job,
  plan,
  banners,
  songs,
  busy,
  onPlanChange,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  banners: StreamBannerActions;
  songs: Song[] | null;
  busy: boolean;
  onPlanChange: (plan: StreamEditPlan) => void;
}): ReactNode {
  const variantMeta = STREAM_VARIANTS.find((v) => v.value === plan.variant) ?? STREAM_VARIANTS[0];
  const setVariant = (variant: StreamVariant) => onPlanChange({ ...plan, variant });
  const setMusicKey = (key: string) => onPlanChange({ ...plan, music: key ? { key, volume: plan.music?.volume } : {} });
  const setMusicVolume = (volume: number) => onPlanChange({ ...plan, music: { key: plan.music?.key, volume } });
  const setGrade = (grade: boolean) => onPlanChange({ ...plan, effects: { grade } });
  const killFeed = plan.kill_feed ?? {};
  const killFeedPosition =
    killFeed.position_y ??
    killFeedDefaultPosition({
      variant: plan.variant,
      gameplayCrop: plan.gameplay_crop ?? { x: 0, y: 0, width: 1, height: 1 },
      width: job.probe?.width ?? 0,
      height: job.probe?.height ?? 0,
    });
  const setKillFeedCentered = (centered: boolean) =>
    onPlanChange({ ...plan, kill_feed: centered ? { ...killFeed, centered } : {} });
  const setKillFeedPosition = (position_y: number) => onPlanChange({ ...plan, kill_feed: { ...killFeed, position_y } });
  const resetKillFeedPosition = () => onPlanChange({ ...plan, kill_feed: { centered: killFeed.centered } });

  return (
    <>
      <StreamLayoutBar variant={plan.variant} disabled={busy} onVariantChange={setVariant} />
      <StreamLayoutStep needsFaceCrop={variantMeta.needsFaceCrop} faceCropReviewed={plan.face_crop_reviewed === true} />
      <StreamKillFeedControls
        centered={killFeed.centered ?? false}
        position={killFeedPosition}
        hasExplicitPosition={killFeed.position_y !== undefined}
        busy={busy}
        onCenteredChange={setKillFeedCentered}
        onPositionChange={setKillFeedPosition}
        onResetPosition={resetKillFeedPosition}
      />
      <details className="rounded-md border border-border-subtle p-3">
        <DisclosureSummary className="font-semibold text-body-sm">Banners · opcional</DisclosureSummary>
        <div className="mt-3 flex flex-col gap-3">
          <StreamBannerControls
            nick={plan.streamer_banner?.nick ?? ''}
            nickValid={STREAMER_NICK_RE.test(plan.streamer_banner?.nick?.trim() ?? '')}
            platform={banners.bannerPlatform}
            position={banners.bannerPosition}
            hasExplicitPosition={plan.streamer_banner?.position_y !== undefined}
            slideEnabled={plan.streamer_banner?.slide_enabled ?? false}
            busy={busy}
            onNickChange={banners.setStreamerNick}
            onPlatformChange={banners.setStreamerPlatform}
            onPositionChange={banners.setStreamerPosition}
            onResetPosition={banners.resetStreamerPosition}
            onSlideChange={banners.setStreamerSlide}
          />
          <StreamKeyDropBannerControls
            family={plan.keydrop_banner?.family ?? ''}
            style={isKeyDropBannerStyle(plan.keydrop_banner?.style) ? plan.keydrop_banner.style : ''}
            code={plan.keydrop_banner?.code ?? ''}
            codeValid={isKeyDropCodeValid(plan.keydrop_banner?.code ?? '')}
            position={banners.keyDropPosition}
            hasExplicitPosition={plan.keydrop_banner?.position_y !== undefined}
            slideEnabled={plan.keydrop_banner?.slide_enabled ?? false}
            startSeconds={banners.keyDropStart}
            endSeconds={banners.keyDropEnd}
            clipDurationSeconds={banners.longestClipSeconds}
            busy={busy}
            onFamilyChange={banners.setKeyDropFamily}
            onStyleChange={banners.setKeyDropStyle}
            onCodeChange={banners.setKeyDropCode}
            onPositionChange={banners.setKeyDropPosition}
            onResetPosition={banners.resetKeyDropPosition}
            onSlideChange={banners.setKeyDropSlide}
            onStartChange={banners.setKeyDropStart}
            onEndChange={banners.setKeyDropEnd}
          />
        </div>
      </details>
      <details className="rounded-md border border-border-subtle p-3">
        <DisclosureSummary className="font-semibold text-body-sm">Música y realce de color · opcional</DisclosureSummary>
        <div className="mt-3 flex flex-col gap-3">
          <StreamMusicCard
            songs={songs}
            musicKey={plan.music?.key ?? ''}
            volume={plan.music?.volume ?? 0.25}
            grade={plan.effects?.grade ?? false}
            busy={busy}
            onMusicKey={setMusicKey}
            onMusicVolume={setMusicVolume}
            onGrade={setGrade}
          />
        </div>
      </details>
    </>
  );
}
