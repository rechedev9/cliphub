import type { StreamEditPlan, StreamerBannerPlatform, StreamJob } from '@/lib/api/streams';
import type { AffiliateFamily } from '@/lib/api/types';
import {
  persistAffiliateFamily,
  selectAffiliateFamily,
  selectAffiliateOff,
  selectAffiliateStyle,
} from '@/lib/affiliate-banner';
import {
  clampKeyDropBannerPosition,
  clampStreamerBannerPosition,
  resolveKeyDropBannerPosition,
  resolveStreamerBannerPosition,
} from '@/lib/stream-preview';
import {
  DEFAULT_KEYDROP_CODE,
  DEFAULT_KEYDROP_END_SECONDS,
  DEFAULT_KEYDROP_START_SECONDS,
  resolveStreamerBannerPlatform,
} from '@/lib/streams/plan';

export function streamBannerActions({
  job,
  plan,
  onPlanChange,
  revealKeyDrop,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  onPlanChange: (plan: StreamEditPlan) => void;
  revealKeyDrop: (start: number, end: number) => void;
}) {
  const bannerPosition = resolveStreamerBannerPosition(plan.variant, plan.streamer_banner?.position_y);
  const bannerPlatform = resolveStreamerBannerPlatform(plan.streamer_banner?.platform, job.source_url);
  const keyDropPosition = resolveKeyDropBannerPosition(plan.keydrop_banner?.position_y);
  const setStreamerNick = (nick: string) =>
    onPlanChange({ ...plan, streamer_banner: { ...plan.streamer_banner, nick, platform: bannerPlatform } });
  const setStreamerPlatform = (platform: StreamerBannerPlatform) =>
    onPlanChange({ ...plan, streamer_banner: { ...plan.streamer_banner, platform } });
  const setStreamerPosition = (position: number) =>
    onPlanChange({
      ...plan,
      streamer_banner: {
        ...plan.streamer_banner,
        position_y: clampStreamerBannerPosition(position),
        platform: bannerPlatform,
      },
    });
  const resetStreamerPosition = () => {
    const { position_y: _position, ...banner } = plan.streamer_banner ?? {};
    onPlanChange({ ...plan, streamer_banner: { ...banner, platform: bannerPlatform } });
  };
  const setStreamerSlide = (slideEnabled: boolean) =>
    onPlanChange({
      ...plan,
      streamer_banner: { ...plan.streamer_banner, slide_enabled: slideEnabled, platform: bannerPlatform },
    });

  const longestClipSeconds = Math.max(0, ...plan.clips.map((c) => Math.max(0, c.end_seconds - c.start_seconds)));
  const keyDropStart = plan.keydrop_banner?.start_seconds ?? DEFAULT_KEYDROP_START_SECONDS;
  const keyDropEndRaw = plan.keydrop_banner?.end_seconds;
  const keyDropEnd =
    keyDropEndRaw ??
    (longestClipSeconds > 0 ? Math.min(DEFAULT_KEYDROP_END_SECONDS, longestClipSeconds) : DEFAULT_KEYDROP_END_SECONDS);

  const setKeyDropStyle = (style: string, family = plan.keydrop_banner?.family ?? '') => {
    if (!style) {
      const off = selectAffiliateOff();
      onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, family: off.family, style: off.style } });
      return;
    }
    const selected = selectAffiliateStyle(family, style);
    const next = {
      ...plan.keydrop_banner,
      family: persistAffiliateFamily(selected.family, selected.style) || selected.family,
      style: selected.style,
    };
    if (!plan.keydrop_banner?.code?.trim()) {
      next.code = DEFAULT_KEYDROP_CODE;
    }
    if (plan.keydrop_banner?.start_seconds === undefined) {
      next.start_seconds = DEFAULT_KEYDROP_START_SECONDS;
    }
    if (plan.keydrop_banner?.end_seconds === undefined) {
      next.end_seconds =
        longestClipSeconds > 0
          ? Math.min(DEFAULT_KEYDROP_END_SECONDS, longestClipSeconds)
          : DEFAULT_KEYDROP_END_SECONDS;
    }
    const start = next.start_seconds ?? DEFAULT_KEYDROP_START_SECONDS;
    const end = next.end_seconds ?? DEFAULT_KEYDROP_END_SECONDS;
    revealKeyDrop(start, end);
    onPlanChange({ ...plan, keydrop_banner: next });
  };
  const setKeyDropFamily = (family: AffiliateFamily) => {
    const selected = selectAffiliateFamily(
      { family: plan.keydrop_banner?.family ?? '', style: plan.keydrop_banner?.style ?? '' },
      family,
    );
    setKeyDropStyle(selected.style, selected.family);
  };
  const setKeyDropCode = (code: string) => {
    revealKeyDrop(keyDropStart, keyDropEnd);
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, code } });
  };
  const setKeyDropPosition = (position: number) =>
    onPlanChange({
      ...plan,
      keydrop_banner: { ...plan.keydrop_banner, position_y: clampKeyDropBannerPosition(position) },
    });
  const resetKeyDropPosition = () => {
    const { position_y: _position, ...banner } = plan.keydrop_banner ?? {};
    onPlanChange({ ...plan, keydrop_banner: banner });
  };
  const setKeyDropSlide = (slideEnabled: boolean) =>
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, slide_enabled: slideEnabled } });
  const setKeyDropStart = (startSeconds: number) => {
    const start = Math.max(0, startSeconds);
    let end = keyDropEnd;
    if (end <= start) end = start + 0.5;
    revealKeyDrop(start, end);
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, start_seconds: start, end_seconds: end } });
  };
  const setKeyDropEnd = (endSeconds: number) => {
    let end = Math.max(0.1, endSeconds);
    if (longestClipSeconds > 0) end = Math.min(end, longestClipSeconds);
    let start = keyDropStart;
    if (end <= start) start = Math.max(0, end - 0.5);
    revealKeyDrop(start, end);
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, start_seconds: start, end_seconds: end } });
  };

  return {
    bannerPosition,
    bannerPlatform,
    keyDropPosition,
    keyDropStart,
    keyDropEnd,
    longestClipSeconds,
    setStreamerNick,
    setStreamerPlatform,
    setStreamerPosition,
    resetStreamerPosition,
    setStreamerSlide,
    setKeyDropFamily,
    setKeyDropStyle,
    setKeyDropCode,
    setKeyDropPosition,
    resetKeyDropPosition,
    setKeyDropSlide,
    setKeyDropStart,
    setKeyDropEnd,
  };
}

export type StreamBannerActions = ReturnType<typeof streamBannerActions>;
