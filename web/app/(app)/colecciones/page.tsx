import type { ReactNode } from 'react';
import { CollectionLibrary } from '@/components/collections/collection-library';
import { StudioPageHeader } from '@/components/studio/page-header';

/** Colecciones: moments saved from the replay, to watch again or turn into a Short. */
export default function CollectionsPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Colecciones"
        description="Momentos guardados desde la repetición con Guardar momento. Reprodúcelos uno tras otro o envíalos a un Short."
      />
      <CollectionLibrary />
    </div>
  );
}
