import type { Video } from './api/types.ts';
import { isHighlightsVideo, OUTPUT_TYPE, outputType } from './clips/hub.ts';
import { customHudLabel, verticalHudTheme } from './custom-hud.ts';
import { prettyMapName } from './format.ts';

export type PublishSummaryFacts = { map: string; player: string; score: string };

/** The HUD the render drew, as the publish page names it; null when the edit does not say. */
export function videoHudLabel(video: Video): string | null {
  const edit = video.editConfig;
  if (edit === undefined) return null;
  if (edit.fullDemo) {
    const theme = edit.fullDemo.document.options.overlays.hud_theme;
    return theme ? `HUD de retransmisión ${customHudLabel(theme)}` : 'HUD nativo de CS2';
  }
  if (edit.nativeHud) return 'HUD nativo de CS2';
  if (edit.verticalHud) return `HUD vertical ${verticalHudTheme(edit.verticalHudTheme).name}`;
  return null;
}

function withExtras(hud: string | null, voice: string): string {
  if (hud) return `, POV con ${hud}${voice ? ` y ${voice}` : ''}`;
  return voice ? `, con ${voice}` : '';
}

/** One line about the video for the publish page, built only from what the render used. */
export function publishSummary(video: Video, facts: PublishSummaryFacts): string {
  const map = facts.map === '' ? '' : prettyMapName(facts.map);
  const where = `${facts.player ? `${facts.player} en ` : ''}${map}${facts.score ? ` (${facts.score})` : ''}`;
  const hud = videoHudLabel(video);
  const edit = video.editConfig;
  if (edit?.fullDemo) {
    const voice = edit.fullDemo.document.voice.availability === 'available' && edit.voiceComms && (edit.voiceVolume ?? 0) > 0;
    return `Rondas de ${where}${withExtras(hud, voice ? 'voces del equipo' : '')}.`;
  }
  if (outputType(video) === OUTPUT_TYPE.short) return `Highlights de ${where}${hud ? `, con ${hud}` : ''}.`;
  const comms = edit?.voiceComms === true ? 'comms' : '';
  if (isHighlightsVideo(video)) return `Mejores jugadas de ${where}${withExtras(hud, comms)}.`;
  return `Partida completa de ${where}${withExtras(hud, comms)}.`;
}
