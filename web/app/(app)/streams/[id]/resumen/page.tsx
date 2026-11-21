'use client';

import { use, type ReactNode } from 'react';
import { StreamSummaryScreen } from '@/components/streams/stream-summary-screen';

export default function StreamSummaryPage({ params }: { params: Promise<{ id: string }> }): ReactNode {
  const { id } = use(params);
  return <StreamSummaryScreen jobId={id} />;
}
