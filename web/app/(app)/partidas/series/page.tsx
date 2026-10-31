import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { SeriesLibrary } from '@/components/series/series-library';

/** Series: the BO1, BO3 and BO5 of the library, with their maps and veto. */
export default function LibrarySeriesPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Series"
        description="Cada serie junta los mapas de un BO1, BO3 o BO5 con sus dos equipos y el orden de picks y bans, si se conoce."
      />
      <SeriesLibrary />
    </div>
  );
}
