import { Suspense, type ReactNode } from 'react';
import { DocumentPage } from '@/components/records/document-page';
import { TeamLibrary } from '@/components/teams/team-library';

/** Equipos: team records built from the clan names of the library's demos. */
export default function TeamsPage(): ReactNode {
  return (
    <DocumentPage
      title="Equipos"
      description="La ficha de cada equipo con su plantilla, sus partidas y su rendimiento por mapa, el punto de partida para preparar un rival."
    >
      <Suspense>
        <TeamLibrary />
      </Suspense>
    </DocumentPage>
  );
}
