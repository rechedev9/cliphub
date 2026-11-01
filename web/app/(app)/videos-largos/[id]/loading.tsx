import type { ReactNode } from 'react';
import { PRODUCE_FORMAT } from '@/lib/clips/routes';
import { ProduceLoading } from '@/components/produce/produce-page';

/** Replaces the generic app skeleton with this page's own frame while the route loads. */
export default function FullDemoLoading(): ReactNode {
  return <ProduceLoading format={PRODUCE_FORMAT.full} seriesId={null} />;
}
