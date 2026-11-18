import { persistAffiliateFamily } from '../affiliate-banner.ts';
import { radarEditFields } from '../radar-overlay.ts';
import { DEFAULT_PRE_ROLL_SECONDS, DEFAULT_VICTIM_REPLAY_SPEED, type EditConfig } from './types.ts';

export type EditRequestBody = {
  full_demo?: EditConfig['fullDemo'];
  format: EditConfig['format'];
  killEffect: EditConfig['killEffect'];
  transition: EditConfig['transition'];
  intro: boolean;
  outro: boolean;
  hook_text: boolean;
  kill_counter: boolean;
  centered_killfeed?: boolean;
  vertical_hud?: boolean;
  vertical_hud_theme?: string;
  radar?: boolean;
  radar_position?: string;
  radar_size?: string;
  radar_opacity?: number;
  match_recap: boolean;
  death_reel?: boolean;
  separate_shorts?: boolean;
  voice_comms: boolean;
  voice_volume?: number;
  subtitles?: boolean;
  subtitles_english?: boolean;
  subtitles_hidden?: string[];
  native_hud: boolean;
  victim_replay?: boolean;
  victim_replay_speed?: number;
  pre_roll_seconds?: number;
  capture_hud?: EditConfig['captureHud'];
  portrait_safe_killfeed?: boolean;
  play_badges?: boolean;
  win_probability?: boolean;
  player_labels?: boolean;
  grenade_trails?: boolean;
  stats_card?: boolean;
  opening_card?: string;
  end_card?: string;
  cover_card?: string;
  brand_watermark?: boolean;
  brand_banner?: boolean;
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
  if (edit.deathReel) body.death_reel = true;
  else if (edit.separateShorts && !edit.matchRecap && !edit.fullDemo) body.separate_shorts = true;
  if (edit.fullDemo) body.full_demo = edit.fullDemo;
  else if (edit.victimReplay && !edit.matchRecap && !edit.deathReel) {
    body.victim_replay = true;
    if (edit.victimReplaySpeed !== undefined && edit.victimReplaySpeed !== DEFAULT_VICTIM_REPLAY_SPEED) body.victim_replay_speed = edit.victimReplaySpeed;
  }
  if (!edit.fullDemo && !edit.matchRecap) {
    if (!edit.deathReel && edit.preRollSeconds !== undefined && edit.preRollSeconds !== DEFAULT_PRE_ROLL_SECONDS) body.pre_roll_seconds = edit.preRollSeconds;
    if (edit.captureHud && edit.format === 'landscape-16x9' && !edit.nativeHud) body.capture_hud = edit.captureHud;
    if (edit.portraitSafeKillfeed && edit.format === 'landscape-16x9' && !edit.deathReel && (edit.nativeHud || edit.captureHud !== 'clean')) body.portrait_safe_killfeed = true;
    if (edit.playBadges && !edit.deathReel) body.play_badges = true;
    if (edit.winProbability && !edit.deathReel) body.win_probability = true;
    if (edit.playerLabels) body.player_labels = true;
    if (edit.grenadeTrails) body.grenade_trails = true;
    if (edit.statsCard && edit.format === 'short-9x16' && !edit.endCard) body.stats_card = true;
    if (edit.format === 'short-9x16') {
      if (edit.openingCard) body.opening_card = edit.openingCard;
      if (edit.endCard) body.end_card = edit.endCard;
      if (edit.coverCard && edit.coverStrategy !== 'no-cover') body.cover_card = edit.coverCard;
    }
    if (edit.brandWatermark) body.brand_watermark = true;
    if (edit.brandBanner && !edit.keyDropStyle?.trim()) body.brand_banner = true;
  }
  const portraitShort = edit.format === 'short-9x16' && !edit.fullDemo && !edit.deathReel;
  const verticalHud = portraitShort && !!edit.verticalHud && !edit.matchRecap && !edit.nativeHud;
  if (verticalHud) {
    body.vertical_hud = true;
    const theme = edit.verticalHudTheme?.trim();
    if (theme) body.vertical_hud_theme = theme;
  }
  // The vertical HUD draws its kill feed under the scoreboard, so it always carries it.
  if ((edit.centeredKillfeed || verticalHud) && portraitShort) body.centered_killfeed = true;
  const radar = radarEditFields(edit);
  if (radar.radar) {
    body.radar = true;
    if (radar.radarPosition) body.radar_position = radar.radarPosition;
    if (radar.radarSize) body.radar_size = radar.radarSize;
    if (radar.radarOpacity !== undefined) body.radar_opacity = radar.radarOpacity;
  }
  if (edit.voiceComms || edit.fullDemo) {
    body.voice_volume = edit.voiceVolume ?? 0.85;
  }
  // Subtitles write what the comms say, so Studio refuses them without the comms.
  if (edit.subtitles && edit.voiceComms && !edit.fullDemo) {
    body.subtitles = true;
    if (edit.subtitlesEnglish) body.subtitles_english = true;
    if (edit.subtitlesHidden?.length) body.subtitles_hidden = [...edit.subtitlesHidden].sort();
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
