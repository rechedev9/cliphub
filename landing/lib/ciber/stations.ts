// Positions are Blender coordinates (Z up, metres), the same numbers as landing/scene/puesto07.py.
export type Vec3 = readonly [number, number, number];

export const STATION_IDS = ["inicio", "short", "demo-completa", "clips", "analisis", "descargar"] as const;
export type StationId = (typeof STATION_IDS)[number];

export interface Shot {
  readonly position: Vec3;
  readonly target: Vec3;
  /** Horizontal field of view in degrees. */
  readonly fov: number;
  /** Width over height of this shot's Cycles still (before overscan); the shot then covers the free region like it. */
  readonly still?: number;
}

export interface Station {
  readonly id: StationId;
  readonly label: string;
  readonly wide: Shot;
  readonly tall: Shot;
  /** Mesh in the GLB that lights up on hover. */
  readonly mesh?: string;
  readonly hotspot?: { readonly center: Vec3; readonly size: Vec3 };
  readonly anchor?: Vec3;
}

export const STATIONS: readonly Station[] = [
  {
    id: "inicio",
    label: "La mesa",
    wide: { position: [0, -1.22, 1.26], target: [0, 0, 1.02], fov: 65.5, still: 1440 / 900 },
    tall: { position: [0, -1.38, 1.62], target: [0, -0.08, 1.12], fov: 56.0, still: 1408 / 1600 },
  },
  {
    id: "short",
    label: "Short",
    wide: { position: [0.06, -0.88, 1.09], target: [0, -0.09, 1.0], fov: 43 },
    tall: { position: [0, -0.8, 1.07], target: [0, -0.08, 1.0], fov: 38 },
    mesh: "screen_main",
    hotspot: { center: [0, -0.06, 1.005], size: [0.44, 0.1, 0.41] },
    anchor: [-0.07, -0.06, 1.215],
  },
  {
    id: "demo-completa",
    label: "Demo completa",
    wide: { position: [-0.2, -0.6, 1.36], target: [-0.445, -0.218, 1.125], fov: 36 },
    tall: { position: [-0.24, -0.66, 1.4], target: [-0.445, -0.218, 1.12], fov: 30 },
    mesh: "cd",
    hotspot: { center: [-0.445, -0.22, 1.125], size: [0.18, 0.18, 0.05] },
    anchor: [-0.45, -0.24, 1.16],
  },
  {
    id: "clips",
    label: "Clips de stream",
    wide: { position: [1.6, -0.15, 1.42], target: [0.95, 1.5, 1.04], fov: 32 },
    tall: { position: [1.36, -0.35, 1.46], target: [0.92, 1.45, 1.02], fov: 36 },
    mesh: "screen_far2",
    hotspot: { center: [0.95, 1.55, 1.03], size: [0.46, 0.12, 0.52] },
    anchor: [0.95, 1.52, 1.235],
  },
  {
    id: "analisis",
    label: "Análisis",
    wide: { position: [-0.2, -0.56, 1.16], target: [-0.3, -0.19, 0.75], fov: 43 },
    tall: { position: [-0.22, -0.6, 1.24], target: [-0.3, -0.19, 0.75], fov: 38 },
    mesh: "notepad",
    hotspot: { center: [-0.3, -0.19, 0.758], size: [0.18, 0.24, 0.025] },
    anchor: [-0.31, -0.2, 0.78],
  },
  {
    id: "descargar",
    label: "Descargar",
    wide: { position: [-0.1, -0.68, 0.98], target: [-0.28, -0.4, 0.75], fov: 36 },
    tall: { position: [-0.12, -0.74, 1.04], target: [-0.26, -0.4, 0.75], fov: 34 },
    mesh: "ticket",
    hotspot: { center: [-0.26, -0.41, 0.755], size: [0.16, 0.13, 0.03] },
    anchor: [-0.27, -0.42, 0.775],
  },
];

export function isStationId(value: string): value is StationId {
  return STATION_IDS.some((id) => id === value);
}

export function stationById(id: StationId): Station {
  return STATIONS.find((s) => s.id === id) ?? STATIONS[0];
}

/** Stations in tour order, without the desk. */
export const TOUR: readonly StationId[] = STATION_IDS.filter((id) => id !== "inicio");
