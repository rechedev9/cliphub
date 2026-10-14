import type { ReactNode } from 'react';
import { FLOW_QUERY, PRODUCE_FORMAT } from '@/lib/clips/routes';
import { DemoFlow, type DemoFlowQuery } from '@/components/produce/demo-flow';

/** Home of the Shorts flow. The key restarts the flow when `?job=` changes. */
export default async function Page({ searchParams }: { searchParams: Promise<DemoFlowQuery> }): Promise<ReactNode> {
  const query = await searchParams;
  return <DemoFlow key={String(query[FLOW_QUERY.job] ?? '')} format={PRODUCE_FORMAT.short} query={query} />;
}
