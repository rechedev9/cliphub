import type { RadarStyle } from '@/components/tactical/radar-draw';

/** Reads the design tokens off the live canvas, so the radar follows the theme. */
export function readRadarStyle(canvas: HTMLCanvasElement): RadarStyle {
  const computed = getComputedStyle(canvas);
  const token = (name: string, fallback: string): string => {
    const value = computed.getPropertyValue(name).trim();
    return value === '' ? fallback : value;
  };
  return {
    // --surface-0 is the ramp's void/well step, so the radar reads as a recess
    // in the --surface-2 panel instead of being painted in the panel's own
    // colour. `panel` is also the health-ring backing, which needs a dark ring
    // under it to separate stacked blips.
    background: token('--surface-0', '#04070f'),
    panel: token('--surface-0', '#04070f'),
    // Identity chips take the ramp's raised step rather than more alpha over
    // --surface-0, so a name reads on an opaque plate whatever is behind it.
    plate: token('--surface-3', '#141b2b'),
    // The occupancy grid is the map, not a signal: it stays neutral so cyan can
    // keep meaning "CT" on top of it.
    map: token('--fg-3', '#8998a7'),
    callout: token('--fg-3', '#8998a7'),
    ct: token('--primary', '#21d9ee'),
    t: token('--warning', '#fcb52c'),
    bomb: token('--destructive', '#fe545c'),
    defuse: token('--success', '#5de0b0'),
    utility: token('--fg-3', '#8998a7'),
    text: token('--fg-1', '#edf7fb'),
    // Resolved through the element, so the var() in --font-mono is already
    // substituted with the family next/font generated.
    fontFamily: computed.fontFamily || 'ui-monospace, monospace',
  };
}

