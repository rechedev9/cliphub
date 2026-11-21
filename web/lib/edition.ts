import { RESULTS_HREF } from './clips/routes.ts';
import { isPathAvailable, type Edition } from './edition-routes.ts';
import { NAV_SECTIONS, sectionForPath, type NavSection } from './nav.ts';

export function homePathFor(edition: Edition): string {
  return edition === 'web' ? '/partidas' : RESULTS_HREF;
}

export function homeLabelFor(edition: Edition): string {
  return edition === 'web' ? 'Partidas' : 'Mis vídeos';
}

export function navSectionsFor(edition: Edition): readonly NavSection[] {
  return NAV_SECTIONS.filter((section) => isPathAvailable(section.href, edition));
}

/** The section a path belongs to, only when the edition serves it. */
export function editionSectionForPath(pathname: string, edition: Edition): NavSection | null {
  const section = sectionForPath(pathname);
  return section !== null && isPathAvailable(section.href, edition) ? section : null;
}
