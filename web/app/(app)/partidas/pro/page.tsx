import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { ProMatchList } from '@/components/pro-matches/pro-match-list';

/** Partidas pro: recent HLTV matches the ClipHub server analysed, ready to open in the Replay. */
export default function ProMatchesPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Partidas pro"
        description="Las partidas de HLTV de los últimos días, ya analizadas por ClipHub. Abre un mapa en el Replay sin descargar la demo, o sigue a un equipo para tener sus partidas nuevas en tu biblioteca."
      />
      <ProMatchList />
    </div>
  );
}
