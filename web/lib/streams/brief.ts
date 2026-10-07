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

/** The stream production choices used by the persisted render plan. `musicLabel` is the catalog title. */
export function streamCreativeBrief(plan: StreamEditPlan, musicLabel?: string): CreativeBriefItem[] {
  const needsFace =
    STREAM_VARIANTS.find((entry) => entry.value === plan.variant)?.needsFaceCrop ?? false;
  const musicKey = plan.music?.key?.trim() ?? '';
  const musicVolume = plan.music?.volume;
  const music =
    musicKey === ''
      ? 'Sin música'
      : `${musicLabel?.trim() || musicKey}${typeof musicVolume === 'number' ? ` · ${Math.round(musicVolume * 100)}%` : ''}`;
  const nick = plan.streamer_banner?.nick?.trim() ?? '';
  const totalOut = plan.clips.reduce((sum, clip) => sum + clipOutputDuration(clip), 0);
  const clipSummary =
    plan.clips.length === 0
      ? 'Sin momentos'
      : `${plan.clips.length} Short${plan.clips.length === 1 ? '' : 's'} · ${formatStreamClock(totalOut)} en total`;

  let facecam = 'Sin cámara';
  if (needsFace) {
    facecam = plan.face_crop_reviewed ? 'Recorte confirmado' : 'Pendiente de confirmar recorte';
  }
  let banner = 'Sin banner';
  if (nick) {
    const platform = plan.streamer_banner?.platform === 'kick' ? 'Kick' : 'Twitch';
    const labeled = `${nick} · ${platform}`;
    banner = plan.streamer_banner?.slide_enabled ? `${labeled} · deslizante` : labeled;
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
        ? ` · de ${typeof start === 'number' ? `${start.toFixed(1)} s` : 'inicio'} a ${typeof end === 'number' ? `${end.toFixed(1)} s` : 'fin'}`
        : '';
    affiliate = plan.keydrop_banner?.slide_enabled
      ? `${familyLabel} · ${styleLabel} · ${kdCode}${window} · deslizante`
      : `${familyLabel} · ${styleLabel} · ${kdCode}${window}`;
  }

  return [
    { label: 'Formato', value: variantLabel(plan.variant) },
    { label: 'Cámara', value: facecam },
    { label: 'Shorts', value: clipSummary },
    { label: 'Banner del streamer', value: banner },
    { label: 'Afiliado', value: affiliate },
    { label: 'Música', value: music },
    { label: 'Realce de color', value: plan.effects?.grade ? 'Sí' : 'No' },
  ];
}
