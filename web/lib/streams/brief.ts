import {
  STREAM_VARIANTS,
  type StreamEditPlan,
  type StreamVariant,
} from '../api/streams.ts';
import { affiliateFamilyLabel, affiliateStyleLabel } from '../api/types.ts';
import type { CreativeBriefItem } from '../reel-brief.ts';
import { clipOutputDuration, formatStreamClock } from './plan.ts';

function variantLabel(variant: StreamVariant): string {
  return STREAM_VARIANTS.find((entry) => entry.value === variant)?.label ?? variant;
}

/** The stream production choices used by the persisted render plan. */
export function streamCreativeBrief(plan: StreamEditPlan): CreativeBriefItem[] {
  const needsFace =
    STREAM_VARIANTS.find((entry) => entry.value === plan.variant)?.needsFaceCrop ?? false;
  const musicKey = plan.music?.key?.trim() ?? '';
  const musicVolume = plan.music?.volume;
  const music =
    musicKey === ''
      ? 'Sin música'
      : `${musicKey}${typeof musicVolume === 'number' ? ` · ${Math.round(musicVolume * 100)}%` : ''}`;
  const nick = plan.streamer_banner?.nick?.trim() ?? '';
  const totalOut = plan.clips.reduce((sum, clip) => sum + clipOutputDuration(clip), 0);
  const clipSummary =
    plan.clips.length === 0
      ? 'Sin clips'
      : `${plan.clips.length} clip${plan.clips.length === 1 ? '' : 's'} · ${formatStreamClock(totalOut)} de salida aprox.`;

  let facecam = 'Sin facecam';
  if (needsFace) {
    facecam = plan.face_crop_reviewed ? 'Recorte confirmado' : 'Pendiente de confirmar recorte';
  }
  let banner = 'Sin nick';
  if (nick) {
    const platform = plan.streamer_banner?.platform === 'kick' ? 'Kick' : 'Twitch';
    const labeled = `${nick} · ${platform}`;
    banner = plan.streamer_banner?.slide_enabled ? `${labeled} · slide` : labeled;
  }
  const kdStyle = plan.keydrop_banner?.style?.trim() ?? '';
  const kdFamily = plan.keydrop_banner?.family?.trim() ?? '';
  const kdCode = (plan.keydrop_banner?.code?.trim() || 'ZACKCSGO').toUpperCase();
  let affiliate = 'No';
  if (kdStyle) {
    const familyLabel = affiliateFamilyLabel(kdFamily, kdStyle);
    const styleLabel = affiliateStyleLabel(kdFamily, kdStyle);
    const start = plan.keydrop_banner?.start_seconds;
    const end = plan.keydrop_banner?.end_seconds;
    const window =
      typeof start === 'number' || typeof end === 'number'
        ? ` · ${typeof start === 'number' ? start.toFixed(1) : '0'}s–${typeof end === 'number' ? end.toFixed(1) : 'fin'}s`
        : '';
    affiliate = plan.keydrop_banner?.slide_enabled
      ? `${familyLabel} · ${styleLabel} · ${kdCode}${window} · slide`
      : `${familyLabel} · ${styleLabel} · ${kdCode}${window}`;
  }

  return [
    { label: 'Layout', value: variantLabel(plan.variant) },
    { label: 'Facecam', value: facecam },
    { label: 'Clips', value: clipSummary },
    { label: 'Banner', value: banner },
    { label: 'Afiliado', value: affiliate },
    { label: 'Música', value: music },
    { label: 'Grade', value: plan.effects?.grade ? 'Sí' : 'No' },
  ];
}
