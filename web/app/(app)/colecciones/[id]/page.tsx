import type { ReactNode } from 'react';
import { CollectionRecord } from '@/components/collections/collection-record';

/** One collection of saved moments. */
export default async function CollectionPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <CollectionRecord collectionId={id} />
    </div>
  );
}
