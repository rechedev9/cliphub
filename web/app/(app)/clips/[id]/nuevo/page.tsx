import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { PRODUCE_QUERY, produceFormatParam } from '@/lib/clips/routes';
import { PRODUCE_DOCUMENT_TITLE } from '@/lib/produce/copy';
import { ProducePage, type ProducePageQuery } from './produce-page';

type Props = {
  params: Promise<{ id: string }>;
  searchParams: Promise<ProducePageQuery>;
};

/** The tab names the format being prepared instead of the parent segment's generic "Partida". */
export async function generateMetadata({ searchParams }: Props): Promise<Metadata> {
  return { title: PRODUCE_DOCUMENT_TITLE[produceFormatParam((await searchParams)[PRODUCE_QUERY.format])] };
}

export default async function Page({ params, searchParams }: Props): Promise<ReactNode> {
  const [{ id }, query] = await Promise.all([params, searchParams]);
  return <ProducePage id={id} query={query} />;
}
