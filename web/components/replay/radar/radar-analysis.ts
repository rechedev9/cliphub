import { TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, hasSampleFlags } from '@/lib/api/tactical';
import type { RadarCalibration, TacticalSample, TacticalSide } from '@/lib/api/tactical';
import type { HitLine } from '@/lib/replay/hit-lines';
import type { FrameSight } from '@/lib/replay/sight-lines';
import { worldToRendered } from '@/lib/tactical-transform';
import type { RadarPoint } from '@/lib/tactical-transform';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { formatDecimal } from '@/lib/format';

/** What the analysis layers add to one frame of the focused round. */
export type FrameAnalysis = { sight: FrameSight | null; hits: readonly HitLine[] };

type Canvas = { ctx: CanvasRenderingContext2D; calibration: RadarCalibration; size: number; style: RadarStyle; samples: readonly TacticalSample[] };

function sideOf(sample: TacticalSample): TacticalSide {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT) ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct;
}

function colorOf(canvas: Canvas, side: TacticalSide): string {
  return side === TACTICAL_SIDES.ct ? canvas.style.ct : canvas.style.t;
}

function placed(canvas: Canvas): Map<number, { at: RadarPoint; side: TacticalSide }> {
  const out = new Map<number, { at: RadarPoint; side: TacticalSide }>();
  for (const sample of canvas.samples) out.set(sample.slot, { at: worldToRendered(canvas.calibration, sample.x, sample.y, canvas.size), side: sideOf(sample) });
  return out;
}

/** Each living player's view across the real field of view, stopped at the first wall. */
export function drawViewWedges(canvas: Canvas, sight: FrameSight): void {
  const { ctx } = canvas;
  const players = placed(canvas);
  for (const [slot, points] of sight.wedges) {
    const player = players.get(slot);
    if (player === undefined || points.length < 2) continue;
    const corners = points.map((point) => worldToRendered(canvas.calibration, point.x, point.y, canvas.size));
    const reach = Math.max(...corners.map((corner) => Math.hypot(corner.x - player.at.x, corner.y - player.at.y)), 1);
    const gradient = ctx.createRadialGradient(player.at.x, player.at.y, 0, player.at.x, player.at.y, reach);
    gradient.addColorStop(0, colorOf(canvas, player.side));
    gradient.addColorStop(1, 'rgba(0, 0, 0, 0)');
    ctx.globalAlpha = 0.24;
    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.moveTo(player.at.x, player.at.y);
    for (const corner of corners) ctx.lineTo(corner.x, corner.y);
    ctx.closePath();
    ctx.fill();
  }
  ctx.globalAlpha = 1;
}

/**
 * A thin line between enemies with nothing between them: white when both look
 * at each other, the watcher's colour when only one does, dashed when neither.
 */
export function drawSightPairs(canvas: Canvas, sight: FrameSight): void {
  const { ctx } = canvas;
  const players = placed(canvas);
  ctx.lineCap = 'round';
  for (const pair of sight.pairs) {
    const a = players.get(pair.a);
    const b = players.get(pair.b);
    if (a === undefined || b === undefined) continue;
    let color = '#f2f4f6';
    if (pair.aSees !== pair.bSees) color = colorOf(canvas, pair.aSees ? a.side : b.side);
    const looked = pair.aSees || pair.bSees;
    ctx.globalAlpha = looked ? 0.85 : 0.4;
    ctx.strokeStyle = color;
    ctx.lineWidth = looked ? 1.5 : 1;
    ctx.setLineDash(looked ? [] : [4, 4]);
    ctx.beginPath();
    ctx.moveTo(a.at.x, a.at.y);
    ctx.lineTo(b.at.x, b.at.y);
    ctx.stroke();
  }
  ctx.setLineDash([]);
  ctx.globalAlpha = 1;
}

/** Attacker to victim, with a head at the victim, fading over the line's second. */
export function drawHitLines(canvas: Canvas, lines: readonly HitLine[]): void {
  const { ctx } = canvas;
  const players = placed(canvas);
  ctx.lineCap = 'round';
  for (const line of lines) {
    const from = players.get(line.from);
    const to = players.get(line.to);
    if (from === undefined || to === undefined) continue;
    const length = Math.hypot(to.at.x - from.at.x, to.at.y - from.at.y);
    if (length < 1) continue;
    const ux = (to.at.x - from.at.x) / length;
    const uy = (to.at.y - from.at.y) / length;
    ctx.globalAlpha = line.alpha;
    ctx.strokeStyle = canvas.style.hits[line.kind];
    ctx.fillStyle = canvas.style.hits[line.kind];
    ctx.lineWidth = line.kind === 'bullet' ? Math.min(3.5, 1.25 + line.amount / 40) : 2;
    ctx.beginPath();
    ctx.moveTo(from.at.x, from.at.y);
    ctx.lineTo(to.at.x, to.at.y);
    ctx.stroke();
    const tipX = to.at.x - ux * 9;
    const tipY = to.at.y - uy * 9;
    ctx.beginPath();
    ctx.moveTo(tipX, tipY);
    ctx.lineTo(tipX - ux * 7 - uy * 4, tipY - uy * 7 + ux * 4);
    ctx.lineTo(tipX - ux * 7 + uy * 4, tipY - uy * 7 - ux * 4);
    ctx.closePath();
    ctx.fill();
  }
  ctx.globalAlpha = 1;
}

/** The damage, or the seconds blinded, beside each victim; drawn over the discs. */
export function drawHitLabels(canvas: Canvas, lines: readonly HitLine[]): void {
  const { ctx } = canvas;
  const players = placed(canvas);
  const stacked = new Map<number, number>();
  ctx.font = `700 11px ${canvas.style.fontFamily}`;
  ctx.textAlign = 'left';
  ctx.textBaseline = 'middle';
  for (const line of lines) {
    const to = players.get(line.to);
    if (to === undefined) continue;
    const row = stacked.get(line.to) ?? 0;
    stacked.set(line.to, row + 1);
    const text = line.kind === 'flash' ? `${formatDecimal(line.amount, 1)} s` : `-${line.amount}${line.headshot ? ' HS' : ''}`;
    const x = to.at.x + 13;
    const y = to.at.y - 12 - row * 13;
    ctx.globalAlpha = line.alpha;
    ctx.lineWidth = 3;
    ctx.strokeStyle = 'rgba(11, 12, 14, 0.9)';
    ctx.strokeText(text, x, y);
    ctx.fillStyle = canvas.style.hits[line.kind];
    ctx.fillText(text, x, y);
  }
  ctx.globalAlpha = 1;
}
