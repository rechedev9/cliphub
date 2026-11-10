import type { PlayerMapRoles, SideMapRole } from '../api/baseline.ts';

/** "T · Lurk de B", or empty without a role on that side. */
export function mapRoleText(side: 'CT' | 'T', role: SideMapRole | undefined): string {
  return role === undefined || role.label === '' ? '' : `${side} · ${role.label}`;
}

/** A card's role line, the map role before the library role: "T · Lurk de B, AWP"; either alone when the other is missing. */
export function cardRoleText(options: { side: 'CT' | 'T'; mapRole: SideMapRole | undefined; role: string }): string {
  return [mapRoleText(options.side, options.mapRole), options.role].filter((part) => part !== '').join(', ');
}

export type MapRoleChip = { side: 't' | 'ct'; label: string };

/** The chips of the sides asked, T first; a side without a role is left out. */
export function mapRoleChips(roles: PlayerMapRoles | undefined, side: '' | 'CT' | 'T'): MapRoleChip[] {
  const chips: MapRoleChip[] = [];
  if (side !== 'CT' && roles?.t !== undefined && roles.t.label !== '') chips.push({ side: 't', label: roles.t.label });
  if (side !== 'T' && roles?.ct !== undefined && roles.ct.label !== '') chips.push({ side: 'ct', label: roles.ct.label });
  return chips;
}
