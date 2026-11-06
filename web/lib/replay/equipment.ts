/** What an equipment icon draws; one silhouette per family. */
export type EquipmentCategory =
  | 'rifle'
  | 'sniper'
  | 'smg'
  | 'heavy'
  | 'pistol'
  | 'smoke'
  | 'flash'
  | 'he'
  | 'molotov'
  | 'decoy'
  | 'c4'
  | 'kit'
  | 'armor'
  | 'helmet'
  | 'taser'
  | 'knife'
  | 'other';

export type EquipmentItem = {
  /** The normalised id, such as `ak47`; the key for `/api/equipment/{id}/icon`. */
  id: string;
  category: EquipmentCategory;
  /** Short readable name, such as `AK-47`. */
  label: string;
};

type Known = { category: EquipmentCategory; label: string };

const KNOWN: Record<string, Known> = {
  ak47: { category: 'rifle', label: 'AK-47' },
  m4a1: { category: 'rifle', label: 'M4A4' },
  m4a4: { category: 'rifle', label: 'M4A4' },
  m4a1silencer: { category: 'rifle', label: 'M4A1-S' },
  m4a1s: { category: 'rifle', label: 'M4A1-S' },
  galilar: { category: 'rifle', label: 'Galil' },
  galil: { category: 'rifle', label: 'Galil' },
  famas: { category: 'rifle', label: 'FAMAS' },
  aug: { category: 'rifle', label: 'AUG' },
  sg556: { category: 'rifle', label: 'SG 553' },
  sg553: { category: 'rifle', label: 'SG 553' },
  awp: { category: 'sniper', label: 'AWP' },
  ssg08: { category: 'sniper', label: 'Scout' },
  scar20: { category: 'sniper', label: 'SCAR-20' },
  g3sg1: { category: 'sniper', label: 'G3SG1' },
  mac10: { category: 'smg', label: 'MAC-10' },
  mp9: { category: 'smg', label: 'MP9' },
  mp7: { category: 'smg', label: 'MP7' },
  mp5sd: { category: 'smg', label: 'MP5-SD' },
  ump45: { category: 'smg', label: 'UMP-45' },
  p90: { category: 'smg', label: 'P90' },
  bizon: { category: 'smg', label: 'PP-Bizon' },
  ppbizon: { category: 'smg', label: 'PP-Bizon' },
  nova: { category: 'heavy', label: 'Nova' },
  xm1014: { category: 'heavy', label: 'XM1014' },
  mag7: { category: 'heavy', label: 'MAG-7' },
  sawedoff: { category: 'heavy', label: 'Sawed-Off' },
  m249: { category: 'heavy', label: 'M249' },
  negev: { category: 'heavy', label: 'Negev' },
  glock: { category: 'pistol', label: 'Glock' },
  glock18: { category: 'pistol', label: 'Glock' },
  usp: { category: 'pistol', label: 'USP-S' },
  usps: { category: 'pistol', label: 'USP-S' },
  uspsilencer: { category: 'pistol', label: 'USP-S' },
  hkp2000: { category: 'pistol', label: 'P2000' },
  p2000: { category: 'pistol', label: 'P2000' },
  p250: { category: 'pistol', label: 'P250' },
  deagle: { category: 'pistol', label: 'Deagle' },
  deserteagle: { category: 'pistol', label: 'Deagle' },
  fiveseven: { category: 'pistol', label: 'Five-SeveN' },
  tec9: { category: 'pistol', label: 'Tec-9' },
  cz75a: { category: 'pistol', label: 'CZ75' },
  cz75auto: { category: 'pistol', label: 'CZ75' },
  elite: { category: 'pistol', label: 'Dual Berettas' },
  dualberettas: { category: 'pistol', label: 'Dual Berettas' },
  revolver: { category: 'pistol', label: 'R8' },
  r8revolver: { category: 'pistol', label: 'R8' },
  smokegrenade: { category: 'smoke', label: 'Humo' },
  smoke: { category: 'smoke', label: 'Humo' },
  flashbang: { category: 'flash', label: 'Flash' },
  flash: { category: 'flash', label: 'Flash' },
  hegrenade: { category: 'he', label: 'HE' },
  he: { category: 'he', label: 'HE' },
  highexplosivegrenade: { category: 'he', label: 'HE' },
  molotov: { category: 'molotov', label: 'Molotov' },
  incgrenade: { category: 'molotov', label: 'Incendiaria' },
  incendiarygrenade: { category: 'molotov', label: 'Incendiaria' },
  incendiary: { category: 'molotov', label: 'Incendiaria' },
  inferno: { category: 'molotov', label: 'Molotov' },
  decoy: { category: 'decoy', label: 'Señuelo' },
  decoygrenade: { category: 'decoy', label: 'Señuelo' },
  c4: { category: 'c4', label: 'C4' },
  c4explosive: { category: 'c4', label: 'C4' },
  defuser: { category: 'kit', label: 'Kit' },
  defusekit: { category: 'kit', label: 'Kit' },
  kevlar: { category: 'armor', label: 'Chaleco' },
  vest: { category: 'armor', label: 'Chaleco' },
  vesthelm: { category: 'helmet', label: 'Chaleco y casco' },
  kevlarhelmet: { category: 'helmet', label: 'Chaleco y casco' },
  taser: { category: 'taser', label: 'Zeus' },
  zeusx27: { category: 'taser', label: 'Zeus' },
  zeus: { category: 'taser', label: 'Zeus' },
  healthshot: { category: 'other', label: 'Jeringuilla' },
};

/** `weapon_m4a1_silencer`, `M4A1-S` and `m4a1silencer` all read as `m4a1silencer`. */
export function equipmentId(raw: string): string {
  return raw.toLowerCase().replace(/^weapon_|^item_/, '').replace(/[^a-z0-9]/g, '');
}

export function equipmentItem(raw: string): EquipmentItem {
  const id = equipmentId(raw);
  const known = KNOWN[id];
  if (known !== undefined) return { id, ...known };
  if (id.startsWith('knife') || id.startsWith('bayonet')) return { id, category: 'knife', label: 'Cuchillo' };
  return { id, category: 'other', label: raw.replace(/^weapon_/i, '') };
}

const ORDER: Record<EquipmentCategory, number> = {
  rifle: 0,
  sniper: 0,
  smg: 0,
  heavy: 0,
  pistol: 1,
  taser: 2,
  he: 3,
  flash: 4,
  smoke: 5,
  molotov: 6,
  decoy: 7,
  c4: 8,
  kit: 9,
  armor: 10,
  helmet: 10,
  other: 11,
  knife: 12,
};

export const GRENADE_CATEGORIES: ReadonlySet<EquipmentCategory> = new Set(['smoke', 'flash', 'he', 'molotov', 'decoy']);

/**
 * What a player row shows: primary, pistol, then each grenade (a second flash
 * stays as its own icon), then the bomb. Knives, armour and kits are left out:
 * the row has its own glyphs for armour and the kit.
 */
export function rowInventory(carried: readonly { item: string; count: number }[]): EquipmentItem[] {
  return carried
    .flatMap((entry) => Array.from({ length: Math.max(1, Math.min(4, entry.count)) }, () => equipmentItem(entry.item)))
    .filter((item) => item.category !== 'knife' && item.category !== 'armor' && item.category !== 'helmet' && item.category !== 'kit')
    .sort((a, b) => ORDER[a.category] - ORDER[b.category]);
}
