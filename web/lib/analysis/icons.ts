import type { EquipmentIconName } from '../api/game-assets.ts';
import type { TeamGrenadeKind } from '../api/grenades.ts';
import type { TacticalBuyType } from '../api/tactical.ts';

/**
 * Which CS2 equipment icon stands for each analysis concept, and how wide each
 * silhouette is, so rows can carry an icon instead of a word and still line up.
 */

/** The weapon that best reads as each buy: what a round of that buy usually carries. */
const BUY_ICON: Readonly<Record<TacticalBuyType, EquipmentIconName | null>> = {
  pistol: 'glock',
  eco: 'p250',
  semi: 'mac10',
  force: 'galilar',
  full: 'ak47',
  unknown: null,
};

export function buyIcon(buy: TacticalBuyType): EquipmentIconName | null {
  return BUY_ICON[buy];
}

const GRENADE_ICON: Readonly<Record<TeamGrenadeKind, EquipmentIconName>> = {
  smoke: 'smokegrenade',
  flash: 'flashbang',
  he: 'hegrenade',
  molotov: 'molotov',
  decoy: 'decoy',
};

export function grenadeIcon(kind: TeamGrenadeKind): EquipmentIconName {
  return GRENADE_ICON[kind];
}

const LONG_GUNS: ReadonlySet<string> = new Set([
  'ak47', 'm4a1', 'm4a1_silencer', 'm4a1_silencer_off', 'galilar', 'famas', 'aug', 'sg556', 'awp', 'ssg08', 'g3sg1',
  'scar20', 'nova', 'xm1014', 'sawedoff', 'mag7', 'm249', 'negev',
]);
const SMGS: ReadonlySet<string> = new Set(['mac10', 'mp9', 'mp7', 'mp5sd', 'ump45', 'p90', 'bizon']);
const GRENADES: ReadonlySet<string> = new Set([
  'hegrenade', 'flashbang', 'smokegrenade', 'decoy', 'molotov', 'incgrenade', 'inferno', 'firebomb',
]);

/** Width over height of the silhouette's box: long guns are wide, grenades narrow. */
export function iconAspect(name: EquipmentIconName): number {
  if (LONG_GUNS.has(name)) return 2.75;
  if (SMGS.has(name)) return 1.75;
  if (GRENADES.has(name)) return 0.7;
  if (name.startsWith('knife') || name === 'bayonet') return 2.2;
  if (name === 'c4' || name === 'planted_c4' || name === 'defuser') return 1.2;
  if (name.includes('armor') || name === 'kevlar' || name === 'assaultsuit' || name === 'helmet') return 1;
  return 1.4;
}
