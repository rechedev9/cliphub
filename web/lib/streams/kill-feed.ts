import type { NormalizedRect, StreamVariant } from '../api/streams.ts';

/** Output geometry of the vertical layouts, mirroring internal/streamclips/variants.go. */
const PANELS: Record<StreamVariant, { faceHeight: number; gameHeight: number }> = {
  'streamer-vertical-stack-40-60': { faceHeight: 768, gameHeight: 1152 },
  'streamer-vertical-stack': { faceHeight: 520, gameHeight: 1400 },
  'streamer-fullframe-nocam': { faceHeight: 0, gameHeight: 1920 },
};

const OUTPUT_WIDTH = 1080;
const OUTPUT_HEIGHT = 1920;
const HUD_BOTTOM_1080 = 155;
const MIN_TOP_EDGE = 8;

/**
 * Where the render puts the first notice when the plan has no position:
 * just below the players HUD on the game panel, as `killFeedTop` in Go.
 */
export function killFeedDefaultPosition(options: {
  variant: StreamVariant;
  gameplayCrop: NormalizedRect;
  width: number;
  height: number;
}): number | null {
  const panel = PANELS[options.variant];
  if (options.width <= 0 || options.height <= 0) return null;
  const w = options.gameplayCrop.width * options.width;
  const h = options.gameplayCrop.height * options.height;
  const scale = Math.max(OUTPUT_WIDTH / w, panel.gameHeight / h);
  const shift = (Math.round(h * scale) - panel.gameHeight) / 2;
  const hud = (HUD_BOTTOM_1080 * (options.height / 1080) - options.gameplayCrop.y * options.height) * scale;
  return (panel.faceHeight + Math.max(MIN_TOP_EDGE, Math.round(hud - shift))) / OUTPUT_HEIGHT;
}
