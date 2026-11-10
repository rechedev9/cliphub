import type { ReactNode } from 'react';
import { DocumentPage } from '@/components/records/document-page';
import { SeriesLibrary } from '@/components/series/series-library';

/** Series: the BO1, BO3 and BO5 of the library, with their maps and veto. */
export default function LibrarySeriesPage(): ReactNode {
  return (
    <DocumentPage
      title="Series"
      description="Cada serie junta los mapas de un BO1, BO3 o BO5 con sus dos equipos y el orden de picks y bans, si se conoce."
    >
      <SeriesLibrary />
    </DocumentPage>
  );
}
