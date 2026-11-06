/**
 * Item ids of the position blob (`tacticalplan.Equipment`), indexed by id. The
 * order is wire format and mirrors `internal/tacticalplan/equipment.go`: new
 * items go at the end, nothing is renumbered. Names are CS2 entity names
 * without the `weapon_` prefix.
 */
export const TACTICAL_EQUIPMENT = [
  '',
  // Pistols
  'glock', 'usp_silencer', 'hkp2000', 'p250', 'elite', 'fiveseven', 'tec9', 'cz75a', 'deagle', 'revolver',
  // SMGs
  'mac10', 'mp9', 'mp7', 'mp5sd', 'ump45', 'p90', 'bizon',
  // Heavy
  'nova', 'xm1014', 'sawedoff', 'mag7', 'm249', 'negev',
  // Rifles
  'galilar', 'famas', 'ak47', 'm4a1', 'm4a1_silencer', 'sg556', 'aug', 'ssg08', 'awp', 'g3sg1', 'scar20',
  // Grenades
  'flashbang', 'smokegrenade', 'hegrenade', 'molotov', 'incgrenade', 'decoy',
  // Other
  'c4', 'knife', 'taser', 'healthshot',
] as const;

/** A known item name; the empty id 0 is not one. */
export type TacticalEquipment = Exclude<(typeof TACTICAL_EQUIPMENT)[number], ''>;

/** One carried item and how many of it, such as two flashbangs. */
export type TacticalInventoryItem = { item: TacticalEquipment; count: number };

const GRENADES: ReadonlySet<TacticalEquipment> = new Set([
  'flashbang', 'smokegrenade', 'hegrenade', 'molotov', 'incgrenade', 'decoy',
]);

/** The item an id names, or null for no item or an id newer than this client. */
export function equipmentById(id: number): TacticalEquipment | null {
  const name = TACTICAL_EQUIPMENT[id];
  return name === undefined || name === '' ? null : name;
}

/** Reports whether an item is a throwable grenade. */
export function isGrenade(item: TacticalEquipment): boolean {
  return GRENADES.has(item);
}
