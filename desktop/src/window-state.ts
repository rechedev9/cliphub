// Pure validation of the persisted window geometry (window.json). Split out of
// loadWindowState() so the finite/size-sanity checks can be unit tested without
// touching the filesystem. No side effects, no relative imports, so `node --test`
// runs this .ts file directly.

interface WindowBounds {
  width: number;
  height: number;
  x?: number;
  y?: number;
}

export interface WindowState {
  bounds: WindowBounds;
  isMaximized: boolean;
}

export interface WorkArea {
  x: number;
  y: number;
  width: number;
  height: number;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

// Same defaults the inline version used: a comfortable windowed size, not
// maximized. Returned as a fresh object each call so callers can never mutate a
// shared fallback.
function fallback(): WindowState {
  return { bounds: { width: 1280, height: 900 }, isMaximized: false };
}

/**
 * Validates an already-JSON.parse'd value read from window.json and returns
 * usable bounds plus the maximize flag, or the fallback if the input is
 * missing, corrupt, the wrong shape, has non-finite dimensions, or is
 * implausibly small. x/y are only carried over when both are finite numbers.
 */
export function validateWindowState(saved: unknown): WindowState {
  if (!isRecord(saved)) return fallback();
  const { width, height, x, y, isMaximized } = saved;
  if (
    typeof width !== 'number' ||
    typeof height !== 'number' ||
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width < 800 ||
    height < 600
  ) {
    return fallback();
  }
  const bounds: WindowBounds = { width, height };
  if (typeof x === 'number' && typeof y === 'number' && Number.isFinite(x) && Number.isFinite(y)) {
    bounds.x = x;
    bounds.y = y;
  }
  return { bounds, isMaximized: isMaximized === true };
}

function usableWorkArea(area: WorkArea): boolean {
  return (
    Number.isFinite(area.x) &&
    Number.isFinite(area.y) &&
    Number.isFinite(area.width) &&
    Number.isFinite(area.height) &&
    area.width > 0 &&
    area.height > 0
  );
}

function overlapArea(bounds: Required<WindowBounds>, area: WorkArea): number {
  const width = Math.max(
    0,
    Math.min(bounds.x + bounds.width, area.x + area.width) - Math.max(bounds.x, area.x),
  );
  const height = Math.max(
    0,
    Math.min(bounds.y + bounds.height, area.y + area.height) - Math.max(bounds.y, area.y),
  );
  return width * height;
}

function centerDistanceSquared(bounds: Required<WindowBounds>, area: WorkArea): number {
  const dx = bounds.x + bounds.width / 2 - (area.x + area.width / 2);
  const dy = bounds.y + bounds.height / 2 - (area.y + area.height / 2);
  return dx * dx + dy * dy;
}

/**
 * Fits persisted bounds into a current display work area. Coordinates remain
 * negative when that is where a real monitor lives; a removed monitor instead
 * selects the nearest remaining work area and moves the complete window back
 * on screen.
 */
export function fitWindowStateToWorkAreas(
  state: WindowState,
  workAreas: readonly WorkArea[],
): WindowState {
  const areas = workAreas.filter(usableWorkArea);
  if (areas.length === 0) return state;

  const { bounds } = state;
  if (bounds.x === undefined || bounds.y === undefined) {
    const primary = areas[0];
    return {
      bounds: {
        width: Math.min(bounds.width, primary.width),
        height: Math.min(bounds.height, primary.height),
      },
      isMaximized: state.isMaximized,
    };
  }

  const positioned = {
    width: bounds.width,
    height: bounds.height,
    x: bounds.x,
    y: bounds.y,
  };
  let target = areas[0];
  let bestOverlap = overlapArea(positioned, target);
  let bestDistance = centerDistanceSquared(positioned, target);
  for (const area of areas.slice(1)) {
    const overlap = overlapArea(positioned, area);
    const distance = centerDistanceSquared(positioned, area);
    if (overlap > bestOverlap || (overlap === bestOverlap && distance < bestDistance)) {
      target = area;
      bestOverlap = overlap;
      bestDistance = distance;
    }
  }

  const width = Math.min(bounds.width, target.width);
  const height = Math.min(bounds.height, target.height);
  const x = Math.min(Math.max(bounds.x, target.x), target.x + target.width - width);
  const y = Math.min(Math.max(bounds.y, target.y), target.y + target.height - height);
  return { bounds: { width, height, x, y }, isMaximized: state.isMaximized };
}
