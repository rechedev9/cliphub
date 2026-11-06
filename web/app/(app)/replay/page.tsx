import { Suspense, type ReactNode } from 'react';
import { PlaylistReplay } from '@/components/replay/playlist-replay';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';

/** A playlist of rounds from any matches, opened through `replayHref`. */
export default function ReplayPage(): ReactNode {
  return (
    <div className="flex flex-col gap-5">
      <Suspense fallback={<TacticalWorkspaceSkeleton />}>
        <PlaylistReplay />
      </Suspense>
    </div>
  );
}
