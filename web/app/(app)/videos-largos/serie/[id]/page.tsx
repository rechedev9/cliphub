import type { ReactNode } from 'react';
import { PRODUCE_FORMAT } from '@/lib/clips/routes';
import { SeriesProgress } from '@/components/series/series-progress';

export default async function Page({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return <SeriesProgress id={id} format={PRODUCE_FORMAT.full} />;
}
