import type { ReactNode } from 'react';
import { PlayerLibrary } from '@/components/players/player-library';
import { DocumentPage } from '@/components/records/document-page';

/** Jugadores: the players of the library, the way in to a player's statistics. */
export default function PlayersPage(): ReactNode {
  return (
    <DocumentPage
      title="Jugadores"
      description="La ficha de cada jugador de la biblioteca, con sus nicks, sus equipos por fecha y sus cifras frente a la media."
    >
      <PlayerLibrary />
    </DocumentPage>
  );
}
