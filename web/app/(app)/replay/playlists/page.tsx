import type { ReactNode } from 'react';
import { PartidasFrame } from '@/components/matches/partidas-frame';
import { PlaylistLibrary } from '@/components/replay/playlist-library';

/** Partidas > Playlists: rounds of any matches, saved in the order the team watches them. */
export default function ReplayPlaylistsPage(): ReactNode {
  return (
    <PartidasFrame>
      <p className="text-body-sm text-fg-3">
        Rondas de cualquier partida, en el orden en que las vais a ver, con un comentario y dibujos en cada una. Ábrelas en el Replay o pásalas al equipo en un fichero.
      </p>
      <PlaylistLibrary />
    </PartidasFrame>
  );
}
