export type MapPlateId =
  | 'ancient'
  | 'anubis'
  | 'dust2'
  | 'inferno'
  | 'mirage'
  | 'nuke'
  | 'overpass'
  | 'vertigo'
  | 'train'
  | 'office'
  | 'italy'
  | 'agency'
  | 'unknown';

export type MapPlate = {
  id: MapPlateId;
  sky: string;
  mid: string;
  ground: string;
  accent: string;
  rim: string;
};

const ALIAS: Record<string, MapPlateId> = {
  ancient: 'ancient',
  anubis: 'anubis',
  dust2: 'dust2',
  dustii: 'dust2',
  inferno: 'inferno',
  mirage: 'mirage',
  nuke: 'nuke',
  overpass: 'overpass',
  vertigo: 'vertigo',
  train: 'train',
  office: 'office',
  italy: 'italy',
  agency: 'agency',
};

const PLATES: Record<Exclude<MapPlateId, 'unknown'>, Omit<MapPlate, 'id'>> = {
  ancient: {
    sky: '#15241c',
    mid: '#2f3d2a',
    ground: '#2a2316',
    accent: '#c4a35a',
    rim: '#8a9a62',
  },
  anubis: {
    sky: '#122428',
    mid: '#1a4244',
    ground: '#b8964a',
    accent: '#3ec4b4',
    rim: '#d4c078',
  },
  dust2: {
    sky: '#2c2818',
    mid: '#8a7038',
    ground: '#c4a050',
    accent: '#f0d080',
    rim: '#e8c878',
  },
  inferno: {
    sky: '#241614',
    mid: '#6a3024',
    ground: '#8a3c28',
    accent: '#e07040',
    rim: '#d48858',
  },
  mirage: {
    sky: '#261e16',
    mid: '#6a5040',
    ground: '#c4a070',
    accent: '#e8c47a',
    rim: '#d4b070',
  },
  nuke: {
    sky: '#161c22',
    mid: '#32383c',
    ground: '#24282c',
    accent: '#d4b030',
    rim: '#8a9094',
  },
  overpass: {
    sky: '#162028',
    mid: '#334654',
    ground: '#3a4a40',
    accent: '#7aa8c0',
    rim: '#90b0c0',
  },
  vertigo: {
    sky: '#101820',
    mid: '#163848',
    ground: '#204858',
    accent: '#4ec8e8',
    rim: '#78d4ec',
  },
  train: {
    sky: '#201816',
    mid: '#4a3830',
    ground: '#322824',
    accent: '#c06040',
    rim: '#c48868',
  },
  office: {
    sky: '#1c2428',
    mid: '#3a484e',
    ground: '#2c3438',
    accent: '#88a8b8',
    rim: '#a0b8c4',
  },
  italy: {
    sky: '#1c2834',
    mid: '#6a5040',
    ground: '#8a6050',
    accent: '#d4a060',
    rim: '#e0b878',
  },
  agency: {
    sky: '#161e20',
    mid: '#2e3c3c',
    ground: '#222a2c',
    accent: '#70a090',
    rim: '#88b4a4',
  },
};

/** Stable map token: strips de_/cs_ and punctuation so "de_dust2" and "Dust2" match. */
export function mapPlateKey(map: string): string {
  return map
    .trim()
    .toLowerCase()
    .replace(/^(de|cs)_/, '')
    .replace(/[^a-z0-9]+/g, '');
}

export function resolveMapPlateId(map: string): MapPlateId {
  return ALIAS[mapPlateKey(map)] ?? 'unknown';
}

export function resolveMapPlate(map: string): MapPlate {
  const id = resolveMapPlateId(map);
  if (id !== 'unknown') return { id, ...PLATES[id] };
  return unknownPlate(mapPlateKey(map) || map);
}

function unknownPlate(seed: string): MapPlate {
  let h = 0x811c9dc5;
  for (let i = 0; i < seed.length; i += 1) {
    h = Math.imul(h ^ seed.charCodeAt(i), 0x01000193);
  }
  h ^= h >>> 16;
  h = Math.imul(h, 0x85ebca6b);
  const hue = 190 + ((h >>> 0) % 141);
  return {
    id: 'unknown',
    sky: `hsl(${hue} 28% 10%)`,
    mid: `hsl(${hue} 32% 18%)`,
    ground: `hsl(${hue} 22% 14%)`,
    accent: `hsl(${hue} 70% 62%)`,
    rim: `hsl(${hue} 50% 48%)`,
  };
}
