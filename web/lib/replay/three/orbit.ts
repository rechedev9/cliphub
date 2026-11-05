/**
 * Camera math for the 3D replay. World units, Z up, as CS2 and the tactical
 * document use them. Azimuth is radians counter-clockwise from +X; elevation is
 * radians above the ground plane.
 */

export type Vec3 = { x: number; y: number; z: number };

export type OrbitState = { target: Vec3; distance: number; azimuth: number; elevation: number };

export type WorldBox = { min: Vec3; max: Vec3 };

export const ORBIT_MIN_ELEVATION = 0.08;
export const ORBIT_MAX_ELEVATION = Math.PI / 2 - 0.02;
export const ORBIT_MIN_DISTANCE = 150;
export const ORBIT_MAX_DISTANCE = 40000;

/** Radians the camera turns per dragged pixel. */
const ORBIT_RADIANS_PER_PIXEL = 0.006;
/** Wheel delta to zoom factor: one notch (100) is about 12%. */
const ZOOM_PER_WHEEL = 0.0012;

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/** Where the camera sits for an orbit state. */
export function orbitPosition(state: OrbitState): Vec3 {
  const ground = Math.cos(state.elevation) * state.distance;
  return {
    x: state.target.x + Math.cos(state.azimuth) * ground,
    y: state.target.y + Math.sin(state.azimuth) * ground,
    z: state.target.z + Math.sin(state.elevation) * state.distance,
  };
}

/** Dragging right turns the map to the right; dragging down tilts towards top-down. */
export function orbitDrag(state: OrbitState, dx: number, dy: number): OrbitState {
  return {
    ...state,
    azimuth: state.azimuth - dx * ORBIT_RADIANS_PER_PIXEL,
    elevation: clamp(state.elevation + dy * ORBIT_RADIANS_PER_PIXEL, ORBIT_MIN_ELEVATION, ORBIT_MAX_ELEVATION),
  };
}

/** Moves the target on the ground plane so the map follows the pointer. */
export function orbitPan(state: OrbitState, dx: number, dy: number, unitsPerPixel: number): OrbitState {
  const rightX = -Math.sin(state.azimuth);
  const rightY = Math.cos(state.azimuth);
  const forwardX = -Math.cos(state.azimuth);
  const forwardY = -Math.sin(state.azimuth);
  return {
    ...state,
    target: {
      x: state.target.x - rightX * dx * unitsPerPixel + forwardX * dy * unitsPerPixel,
      y: state.target.y - rightY * dx * unitsPerPixel + forwardY * dy * unitsPerPixel,
      z: state.target.z,
    },
  };
}

/** A zoom factor for a wheel delta: positive deltas move away. */
export function wheelFactor(deltaY: number): number {
  return Math.exp(clamp(deltaY, -400, 400) * ZOOM_PER_WHEEL);
}

export function orbitZoom(state: OrbitState, deltaY: number): OrbitState {
  return { ...state, distance: clamp(state.distance * wheelFactor(deltaY), ORBIT_MIN_DISTANCE, ORBIT_MAX_DISTANCE) };
}

/** World units one screen pixel covers at the target, for a vertical field of view in degrees. */
export function unitsPerPixelAt(distance: number, fovDegrees: number, viewportHeight: number): number {
  const height = 2 * distance * Math.tan((fovDegrees * Math.PI) / 360);
  return height / Math.max(1, viewportHeight);
}

/** The orbit that frames a box from the south-west at a 50 degree tilt. */
export function framingOrbit(box: WorldBox, fovDegrees: number, aspect: number): OrbitState {
  const sx = box.max.x - box.min.x;
  const sy = box.max.y - box.min.y;
  const sz = box.max.z - box.min.z;
  const radius = Math.max(1, 0.5 * Math.hypot(sx, sy, sz));
  const vertical = (fovDegrees * Math.PI) / 180;
  const horizontal = 2 * Math.atan(Math.tan(vertical / 2) * Math.max(0.1, aspect));
  const half = Math.min(vertical, horizontal) / 2;
  const distance = clamp((radius / Math.sin(half)) * 0.92, ORBIT_MIN_DISTANCE, ORBIT_MAX_DISTANCE);
  return {
    target: { x: box.min.x + sx / 2, y: box.min.y + sy / 2, z: box.min.z + sz * 0.25 },
    distance,
    azimuth: (-3 * Math.PI) / 4,
    elevation: (50 * Math.PI) / 180,
  };
}

/** The top-down view: a centre and half the visible height in world units. */
export type TopState = { x: number; y: number; halfHeight: number };

export function framingTop(box: WorldBox, aspect: number): TopState {
  const sx = box.max.x - box.min.x;
  const sy = box.max.y - box.min.y;
  return {
    x: box.min.x + sx / 2,
    y: box.min.y + sy / 2,
    halfHeight: Math.max(100, Math.max(sy / 2, sx / 2 / Math.max(0.1, aspect)) * 1.06),
  };
}

/** Screen Y grows downward, world Y grows up the screen in the top-down view. */
export function topPan(state: TopState, dx: number, dy: number, viewportHeight: number): TopState {
  const perPixel = (2 * state.halfHeight) / Math.max(1, viewportHeight);
  return { ...state, x: state.x - dx * perPixel, y: state.y + dy * perPixel };
}

export function topZoom(state: TopState, deltaY: number): TopState {
  return { ...state, halfHeight: clamp(state.halfHeight * wheelFactor(deltaY), 80, 30000) };
}
