import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { RadarCalibration, RadarLevel, TacticalSide } from '@/lib/api/tactical';
import type { OverlayRound } from '@/lib/api/spatial';
import { worldToRendered } from '@/lib/tactical-transform';
import { floorAlpha } from '@/lib/analysis/floors';
import { trackPointAt } from '@/lib/heat-live';
import type { RadarStyle } from '@/components/tactical/radar-draw';

/** A dead player's cross stays this long after the death, then the map is the heat's again. */
const DEAD_SECONDS = 4;

export type HeatPlayersFrame = {
  ctx: CanvasRenderingContext2D;
  size: number;
  style: RadarStyle;
  level: RadarLevel;
  calibration: RadarCalibration;
  rounds: readonly OverlayRound[];
  side: TacticalSide;
  seconds: number;
  step: number;
  /** Both floors side by side: a player on the other floor is drawn on its own plate only. */
  split: boolean;
};

/** The followed players of every round at one second of the shared clock, over the heat. */
export function paintHeatPlayers(frame: HeatPlayersFrame): void {
  const { ctx, size, style, level, calibration, rounds, side, seconds, step, split } = frame;
  const floor = { calibration, level, offFloorAlpha: split ? 0 : undefined };
  const dot = Math.max(3, size * 0.0075);
  const color = side === TACTICAL_SIDES.t ? style.t : style.ct;
  ctx.lineWidth = Math.max(1.25, size * 0.0022);
  for (const round of rounds) {
    if (seconds > round.duration_seconds) continue;
    for (const track of round.tracks) {
      if (track.side !== side) continue;
      const at = trackPointAt(track, seconds, step);
      if (at !== undefined) {
        const p = worldToRendered(calibration, at[0], at[1], size);
        ctx.globalAlpha = floorAlpha(floor, at[2]);
        ctx.beginPath();
        ctx.arc(p.x, p.y, dot, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.fill();
        ctx.strokeStyle = style.background;
        ctx.stroke();
        continue;
      }
      const deathAt = (track.start + track.points.length - 1) * step;
      const last = track.points[track.points.length - 1];
      if (!track.died || last === undefined || seconds < deathAt || seconds - deathAt > DEAD_SECONDS) continue;
      const p = worldToRendered(calibration, last[0], last[1], size);
      const r = dot * 0.8;
      ctx.globalAlpha = 0.85 * floorAlpha(floor, last[2]) * (1 - (seconds - deathAt) / DEAD_SECONDS);
      ctx.strokeStyle = color;
      ctx.beginPath();
      ctx.moveTo(p.x - r, p.y - r);
      ctx.lineTo(p.x + r, p.y + r);
      ctx.moveTo(p.x + r, p.y - r);
      ctx.lineTo(p.x - r, p.y + r);
      ctx.stroke();
    }
  }
  ctx.globalAlpha = 1;
}
