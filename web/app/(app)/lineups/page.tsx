import type { ReactNode } from 'react';
import { LineupMaps } from '@/components/pro-lineups/lineup-maps';
import { DocumentPage } from '@/components/records/document-page';

/** Lineups pro: the grenades pros repeat, map by map. */
export default function LineupsPage(): ReactNode {
  return (
    <DocumentPage title="Lineups pro" description="Humos, flashes, molotovs y HE que los pros repiten, leídos de sus demos.">
      <LineupMaps />
    </DocumentPage>
  );
}
