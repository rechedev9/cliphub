import { persistAffiliateFamily } from '../affiliate-banner.ts';
import type { EditConfig } from './types.ts';

export type EditRequestBody = {
  full_demo?: EditConfig['fullDemo'];
  format: EditConfig['format'];
  killEffect: EditConfig['killEffect'];
  transition: EditConfig['transition'];
  intro: boolean;
  outro: boolean;
  hook_text: boolean;
  kill_counter: boolean;
  match_recap: boolean;
  voice_comms: boolean;
  voice_volume?: number;
  native_hud: boolean;
  cover_strategy: EditConfig['coverStrategy'];
  intro_text?: string;
  outro_text?: string;
  keydrop_family?: string;
  keydrop_style?: string;
  keydrop_code?: string;
  keydrop_position_y?: number;
  keydrop_start_seconds?: number;
  keydrop_end_seconds?: number;
  demo_source?: EditConfig['demoSource'];
  overlay_theme?: EditConfig['overlayTheme'];
};

export function buildEditRequest(edit: EditConfig): EditRequestBody {
  const body: EditRequestBody = {
    format: edit.format,
    killEffect: edit.killEffect,
    transition: edit.transition,
    intro: edit.intro,
    outro: edit.outro,
    hook_text: edit.hookText,
    kill_counter: edit.killCounter,
    match_recap: edit.matchRecap,
    voice_comms: edit.voiceComms,
    native_hud: edit.nativeHud,
    cover_strategy: edit.coverStrategy,
  };
  if (edit.fullDemo) body.full_demo = edit.fullDemo;
  if (edit.voiceComms || edit.fullDemo) {
    body.voice_volume = edit.voiceVolume ?? 0.85;
  }
  const introText = edit.introText?.trim();
  if (edit.intro && introText) body.intro_text = introText;
  const outroText = edit.outroText?.trim();
  if (edit.outro && outroText) body.outro_text = outroText;
  const keyDropStyle = edit.keyDropStyle?.trim();
  if (keyDropStyle) {
    const family = persistAffiliateFamily(edit.keyDropFamily ?? '', keyDropStyle);
    if (family) body.keydrop_family = family;
    body.keydrop_style = keyDropStyle;
    const code = edit.keyDropCode?.trim();
    if (code) body.keydrop_code = code.toUpperCase();
    if (typeof edit.keyDropPositionY === 'number') {
      body.keydrop_position_y = edit.keyDropPositionY;
    }
    if (typeof edit.keyDropStartSeconds === 'number' && Number.isFinite(edit.keyDropStartSeconds)) {
      body.keydrop_start_seconds = edit.keyDropStartSeconds;
    }
    if (typeof edit.keyDropEndSeconds === 'number' && Number.isFinite(edit.keyDropEndSeconds)) {
      body.keydrop_end_seconds = edit.keyDropEndSeconds;
    }
  }
  if (edit.demoSource) {
    body.demo_source = edit.demoSource;
  }
  if (edit.overlayTheme) {
    body.overlay_theme = edit.overlayTheme;
  }
  return body;
}

export function editConfigsEqual(left: EditConfig, right: EditConfig): boolean {
  if (left.fullDemo || right.fullDemo) {
    return !!left.fullDemo && !!right.fullDemo && left.fullDemo.document.plan_hash === right.fullDemo.document.plan_hash;
  }
  return JSON.stringify(buildEditRequest(left)) === JSON.stringify(buildEditRequest(right));
}
