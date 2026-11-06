/**
 * CS2's own art, read at runtime from the user's game install (`internal/gameassets`), so Studio
 * never ships Valve files: overview radars as PNG and equipment icons as sanitized SVG.
 *
 * Radars are 1024x1024 and match `internal/radarmap`'s calibration (`pos_x`, `pos_y`, `scale`),
 * so a world position lands on `((x - pos_x) / scale, (pos_y - y) / scale)` pixels. Nuke, vertigo
 * and train also ship a `lower` layer; the calibration splits floors for nuke and vertigo only.
 * Icons are white silhouettes on a transparent background, drawn for dark surfaces.
 */

/** `upper` is the main overview; `lower` the bottom floor of a two-level map. */
export type RadarLayer = 'upper' | 'lower';

/** Same rule as the orchestrator's `geometryMapPattern`. */
export const RADAR_MAP_NAME_RE = /^(de|cs|ar)_[a-z0-9_]{1,32}$/;

/** Same rule as the orchestrator's `equipmentIconPattern`, plus its 40-character cap. */
export const EQUIPMENT_ICON_NAME_RE = /^[a-z0-9]+(_[a-z0-9]+){0,4}$/;
const MAX_EQUIPMENT_ICON_NAME = 40;

export function isRadarLayer(value: string): value is RadarLayer {
  return value === 'upper' || value === 'lower';
}

export function isEquipmentIconName(value: string): boolean {
  return value.length <= MAX_EQUIPMENT_ICON_NAME && EQUIPMENT_ICON_NAME_RE.test(value);
}

/** Icons CS2 ships under `panorama/images/icons/equipment` (build 2000942). */
export const EQUIPMENT_ICON_NAMES = [
  // Pistols
  'glock', 'hkp2000', 'p2000', 'usp_silencer', 'usp_silencer_off', 'p250', 'elite', 'fiveseven', 'tec9', 'cz75a',
  'deagle', 'revolver',
  // SMGs
  'mac10', 'mp9', 'mp7', 'mp5sd', 'ump45', 'p90', 'bizon',
  // Heavy
  'nova', 'xm1014', 'sawedoff', 'mag7', 'm249', 'negev',
  // Rifles
  'galilar', 'famas', 'ak47', 'm4a1', 'm4a1_silencer', 'm4a1_silencer_off', 'ssg08', 'sg556', 'aug', 'awp', 'g3sg1',
  'scar20',
  // Grenades and fire
  'hegrenade', 'flashbang', 'smokegrenade', 'decoy', 'molotov', 'incgrenade', 'inferno', 'firebomb',
  // Gear
  'kevlar', 'assaultsuit', 'armor', 'armor_helmet', 'helmet', 'heavy_armor', 'defuser', 'c4', 'planted_c4', 'taser',
  'healthshot',
  // Knives
  'knife', 'knife_t', 'knifegg', 'bayonet', 'knife_bowie', 'knife_butterfly', 'knife_canis', 'knife_cord', 'knife_css',
  'knife_falchion', 'knife_flip', 'knife_gut', 'knife_gypsy_jackknife', 'knife_karambit', 'knife_kukri',
  'knife_m9_bayonet', 'knife_outdoor', 'knife_push', 'knife_skeleton', 'knife_stiletto', 'knife_survival_bowie',
  'knife_tactical', 'knife_twinblade', 'knife_ursus', 'knife_widowmaker',
  // Other damage sources
  'world',
] as const;

export type EquipmentIconName = (typeof EQUIPMENT_ICON_NAMES)[number];

const KNOWN_ICONS: ReadonlySet<string> = new Set(EQUIPMENT_ICON_NAMES);

function isKnownIcon(value: string): value is EquipmentIconName {
  return KNOWN_ICONS.has(value);
}

/**
 * Names a tactical event's `weapon` can take besides an icon name: the parser writes the entity
 * name without `weapon_` (see `internal/parser` equipmentNames), and falls back to demoinfocs'
 * display name in lowercase for gear it does not map.
 */
const WEAPON_ICON_ALIASES: Readonly<Record<string, EquipmentIconName>> = {
  m4a4: 'm4a1',
  m4a1s: 'm4a1_silencer',
  usp: 'usp_silencer',
  usps: 'usp_silencer',
  'usp-s': 'usp_silencer',
  glock18: 'glock',
  'glock-18': 'glock',
  'desert eagle': 'deagle',
  'dual berettas': 'elite',
  'five-seven': 'fiveseven',
  'cz75 auto': 'cz75a',
  'r8 revolver': 'revolver',
  galil: 'galilar',
  'galil ar': 'galilar',
  'sg 553': 'sg556',
  sg553: 'sg556',
  scout: 'ssg08',
  'ssg 08': 'ssg08',
  'pp-bizon': 'bizon',
  'mp5-sd': 'mp5sd',
  'he grenade': 'hegrenade',
  'smoke grenade': 'smokegrenade',
  'decoy grenade': 'decoy',
  'incendiary grenade': 'incgrenade',
  incendiary: 'incgrenade',
  flash: 'flashbang',
  smoke: 'smokegrenade',
  he: 'hegrenade',
  'zeus x27': 'taser',
  zeus: 'taser',
  'kevlar vest': 'kevlar',
  vest: 'kevlar',
  'kevlar + helmet': 'assaultsuit',
  vesthelm: 'assaultsuit',
  'defuse kit': 'defuser',
  bomb: 'c4',
  'knife_default_ct': 'knife',
  'knife_default_t': 'knife_t',
};

/**
 * The icon for a tactical event's `weapon` (`ak47`, `weapon_m4a1_silencer`, `Kevlar Vest`), or
 * null when CS2 has no icon for it. Unknown knife skins fall back to the default knife.
 */
export function equipmentIconForWeapon(weapon: string | undefined): EquipmentIconName | null {
  if (weapon === undefined) return null;
  const name = weapon.trim().toLowerCase().replace(/^weapon_/, '');
  if (isKnownIcon(name)) return name;
  const alias = WEAPON_ICON_ALIASES[name];
  if (alias !== undefined) return alias;
  if (name.startsWith('knife') || name.includes('bayonet')) return 'knife';
  return null;
}

/** URL of the map's overview radar for a layer, served by `GET /api/maps/{map}/radar`. */
export function radarImageUrl(map: string, layer: RadarLayer = 'upper'): string {
  return `/api/maps/${encodeURIComponent(map)}/radar?layer=${layer}`;
}

/** URL of an equipment icon, served by `GET /api/equipment/{name}/icon`. */
export function equipmentIconUrl(name: EquipmentIconName): string {
  return `/api/equipment/${encodeURIComponent(name)}/icon`;
}
