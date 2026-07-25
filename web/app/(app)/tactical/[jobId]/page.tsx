import { Suspense } from 'react';
import { TacticalWorkspace } from '@/components/tactical/tactical-workspace';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';

/**
 * The analysis workspace for one demo. The route stays a server component so the
 * `"use client"` boundary sits on the workspace itself, and the Suspense
 * boundary is what lets the client tree read the filter out of the query string.
 */
export default async function TacticalJobPage({
  params,
}: {
  params: Promise<{ jobId: string }>;
}) {
  const { jobId } = await params;
  return (
    <Suspense fallback={<TacticalWorkspaceSkeleton />}>
      <TacticalWorkspace jobId={jobId} />
    </Suspense>
  );
}
