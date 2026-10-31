import type { ReactNode } from 'react';
import { TeamRecord } from '@/components/teams/team-record';

/** One team's record. */
export default async function TeamPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <TeamRecord teamId={id} />
    </div>
  );
}
