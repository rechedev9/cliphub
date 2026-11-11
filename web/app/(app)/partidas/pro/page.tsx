import { Suspense, type ReactNode } from 'react';
import { PartidasFrame } from '@/components/matches/partidas-frame';
import { ProMatchList } from '@/components/pro-matches/pro-match-list';
import { TableSkeleton } from '@/components/records/table-skeleton';

/** Partidas > Públicas: recent HLTV matches the ClipHub server analysed, ready to open in the Replay. */
export default function ProMatchesPage(): ReactNode {
  return (
    <PartidasFrame>
      <p className="text-body-sm text-fg-3">
        Las partidas de HLTV de los últimos días, ya analizadas por ClipHub. Abre un mapa en el Replay sin descargar la demo, o sigue a un equipo para tener sus partidas nuevas en tu biblioteca.
      </p>
      <Suspense fallback={<TableSkeleton label="Cargando partidas pro" columns={3} />}>
        <ProMatchList />
      </Suspense>
    </PartidasFrame>
  );
}
