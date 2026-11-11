import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { RadarCalibration, RadarLevel, TacticalSide } from '@/lib/api/tactical';
import type { OverlayRound } from '@/lib/api/spatial';
import { worldToRendered } from '@/lib/tactical-transform';
import { levelAlpha } from '@/lib/analysis/floors';
import { trackLevels } from '@/lib/heat-live';
import { trackDeathSeconds, trackPointAt } from '@/lib/tactical-spatial';
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

/** The followed players of every round at one second of the shared clock, over the heat, on the replay's floors. */
export function paintHeatPlayers(frame: HeatPlayersFrame): void {
  const { ctx, size, style, level, calibration, rounds, side, seconds, step, split } = frame;
  const floor = { level, offFloorAlpha: split ? 0 : undefined };
  const dot = Math.max(3, size * 0.0075);
  const color = side === TACTICAL_SIDES.t ? style.t : style.ct;
  ctx.lineWidth = Math.max(1.25, size * 0.0022);
  for (const round of rounds) {
    if (seconds > round.duration_seconds) continue;
    for (const track of round.tracks) {
      if (track.side !== side) continue;
      const levels = trackLevels({ track, step, tickrate: round.tickrate, calibration });
      const at = trackPointAt(track, seconds, step);
      if (at !== undefined) {
        const p = worldToRendered(calibration, at[0], at[1], size);
        ctx.globalAlpha = levelAlpha(floor, levels[at[3]]);
        ctx.beginPath();
        ctx.arc(p.x, p.y, dot, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.fill();
        ctx.strokeStyle = style.background;
        ctx.stroke();
        continue;
      }
      const deathAt = trackDeathSeconds(track, step);
      const last = track.points.length - 1;
      if (deathAt === null || seconds < deathAt || seconds - deathAt > DEAD_SECONDS) continue;
      const p = worldToRendered(calibration, track.points[last][0], track.points[last][1], size);
      const r = dot * 0.8;
      ctx.globalAlpha = 0.85 * levelAlpha(floor, levels[last]) * (1 - (seconds - deathAt) / DEAD_SECONDS);
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
