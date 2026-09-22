import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { PRODUCE_FORMAT, PRODUCE_QUERY } from '@/lib/clips/routes';
import { PRODUCE_DOCUMENT_TITLE } from '@/lib/produce/copy';
import { ProducePage, type ProducePageQuery } from './produce-page';

type Props = {
  params: Promise<{ id: string }>;
  searchParams: Promise<ProducePageQuery>;
};

/** The tab names the format being prepared instead of the parent segment's generic "Partida". */
export async function generateMetadata({ searchParams }: Props): Promise<Metadata> {
  const format = (await searchParams)[PRODUCE_QUERY.format];
  return { title: format === PRODUCE_FORMAT.full ? PRODUCE_DOCUMENT_TITLE.full : PRODUCE_DOCUMENT_TITLE.short };
}

export default async function Page({ params, searchParams }: Props): Promise<ReactNode> {
  const [{ id }, query] = await Promise.all([params, searchParams]);
  return <ProducePage id={id} query={query} />;
}
