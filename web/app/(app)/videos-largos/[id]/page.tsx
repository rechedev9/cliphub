import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { PRODUCE_FORMAT } from '@/lib/clips/routes';
import { PRODUCE_DOCUMENT_TITLE } from '@/lib/produce/copy';
import { ProducePage, type ProducePageQuery } from '@/components/produce/produce-page';

type Props = {
  params: Promise<{ id: string }>;
  searchParams: Promise<ProducePageQuery>;
};

export const metadata: Metadata = { title: PRODUCE_DOCUMENT_TITLE.full };

export default async function Page({ params, searchParams }: Props): Promise<ReactNode> {
  const [{ id }, query] = await Promise.all([params, searchParams]);
  return <ProducePage id={id} format={PRODUCE_FORMAT.full} query={query} />;
}
