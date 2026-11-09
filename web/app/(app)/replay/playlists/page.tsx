import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { PlaylistLibrary } from '@/components/replay/playlist-library';

/** Replay > Playlists: rounds of any matches, saved in the order the team watches them. */
export default function ReplayPlaylistsPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Playlists"
        description="Rondas de cualquier partida, en el orden en que las vais a ver, con un comentario y dibujos en cada una. Ábrelas en el Replay o pásalas al equipo en un fichero."
      />
      <PlaylistLibrary />
    </div>
  );
}
