import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { TeamLibrary } from '@/components/teams/team-library';

/** Equipos: team records built from the clan names of the library's demos. */
export default function TeamsPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Equipos"
        description="La ficha de cada equipo con su plantilla, sus partidas y su rendimiento por mapa, el punto de partida para preparar un rival."
      />
      <TeamLibrary />
    </div>
  );
}
