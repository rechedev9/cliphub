import type { PlayerMapRoles, SideMapRole } from '../api/baseline.ts';

/** "T · Lurk de B", or empty without a role on that side. */
export function mapRoleText(side: 'CT' | 'T', role: SideMapRole | undefined): string {
  return role === undefined || role.label === '' ? '' : `${side} · ${role.label}`;
}

/** A card's role line, the map role before the library role: "T · Lurk de B, AWP"; either alone when the other is missing. */
export function cardRoleText(options: { side: 'CT' | 'T'; mapRole: SideMapRole | undefined; role: string }): string {
  return [mapRoleText(options.side, options.mapRole), options.role].filter((part) => part !== '').join(', ');
}

/** The roles of the sides asked, T first: "T · Lurk de B, CT · Ancla A"; empty without any. */
export function mapRolesText(roles: PlayerMapRoles | undefined, side: '' | 'CT' | 'T'): string {
  if (roles === undefined) return '';
  const parts: string[] = [];
  if (side !== 'CT') parts.push(mapRoleText('T', roles.t));
  if (side !== 'T') parts.push(mapRoleText('CT', roles.ct));
  return parts.filter((part) => part !== '').join(', ');
}
