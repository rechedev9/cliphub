import type { ReactNode } from 'react';
import { LineupMaps } from '@/components/pro-lineups/lineup-maps';
import { StudioPageHeader } from '@/components/studio/page-header';

/** Lineups pro: the grenades pros repeat, map by map. */
export default function LineupsPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader title="Lineups pro" description="Humos, flashes, molotovs y HE que los pros repiten, leídos de sus demos. Elige un mapa." />
      <LineupMaps />
    </div>
  );
}
