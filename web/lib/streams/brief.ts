import {
  STREAM_VARIANTS,
  type StreamEditPlan,
  type StreamVariant,
} from '../api/streams.ts';
import type { CreativeBriefItem } from '../reel-brief.ts';
import { clipOutputDuration } from './plan.ts';

export function canCreateStreamShorts({
  briefApproved,
  busy,
}: {
  briefApproved: boolean;
  busy: boolean;
}): boolean {
  return !busy && briefApproved;
}

function variantLabel(variant: StreamVariant): string {
  return STREAM_VARIANTS.find((entry) => entry.value === variant)?.label ?? variant;
}

/** Exact, reviewable stream production choices that must be approved before render. */
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
      : `${plan.clips.length} clip${plan.clips.length === 1 ? '' : 's'} · ~${totalOut.toFixed(1)}s salida`;

  let facecam = 'Sin facecam';
  if (needsFace) {
    facecam = plan.face_crop_reviewed ? 'Recorte confirmado' : 'Pendiente de confirmar recorte';
  }
  let banner = 'Sin nick';
  if (nick) {
    banner = plan.streamer_banner?.slide_enabled ? `${nick} · slide` : nick;
  }
  const kdStyle = plan.keydrop_banner?.style?.trim() ?? '';
  const kdCode = (plan.keydrop_banner?.code?.trim() || 'ZACKCSGO').toUpperCase();
  let keydrop = 'No';
  if (kdStyle) {
    const styleLabel = kdStyle === 'classic' ? 'Classic' : 'Operator';
    keydrop = plan.keydrop_banner?.slide_enabled
      ? `${styleLabel} · ${kdCode} · slide`
      : `${styleLabel} · ${kdCode}`;
  }

  return [
    { label: 'Layout', value: variantLabel(plan.variant) },
    { label: 'Facecam', value: facecam },
    { label: 'Clips', value: clipSummary },
    { label: 'Banner', value: banner },
    { label: 'KeyDrop', value: keydrop },
    { label: 'Música', value: music },
    { label: 'Grade', value: plan.effects?.grade ? 'Sí' : 'No' },
  ];
}
