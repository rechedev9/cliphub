import { redirect } from 'next/navigation';
import { PRODUCE_FORMAT, produceHref } from '@/lib/clips/routes';
import { isSeriesId } from '@/lib/series-status';

/** Retired: the Full POV brief now lives under the partida in 01. */
export default async function RetiredFullDemoPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ series?: string | string[] }>;
}): Promise<never> {
  const { id } = await params;
  const { series } = await searchParams;
  const seriesId = typeof series === 'string' && isSeriesId(series) ? series : undefined;
  redirect(produceHref(id, PRODUCE_FORMAT.full, seriesId));
}
