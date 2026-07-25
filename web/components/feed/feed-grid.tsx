import type { FeedItem } from '@/lib/api/types';
import { Skeleton } from '@/components/ui/skeleton';
import { FeedCard } from './feed-card';

/** How many cards carry a rank marker while a ranked sort is active. */
const RANKED_POSITIONS = 3;

/**
 * Auto-fill, keyed to the content container rather than the viewport. The old
 * `sm:grid-cols-2 xl:grid-cols-3` counted viewport pixels while the real column
 * is the viewport minus a 240px sidebar and the shell gutters, so `xl:` fired
 * at 1280 and produced three ~168px cards holding a thumbnail, a two-line title,
 * an avatar and a 44px control. Auto-fill has no such opinion: it fits whatever
 * the container can actually hold.
 */
const FEED_GRID_CLASS = 'grid grid-cols-[repeat(auto-fill,minmax(min(100%,15rem),1fr))] gap-5';

export type FeedGridProps = {
  items: FeedItem[];
  /** Number the leading cards, so a ranked sort visibly changes the page. */
  showRank?: boolean;
};

/** A responsive, portrait-biased grid of community reels. */
export function FeedGrid({ items, showRank = false }: FeedGridProps) {
  return (
    <section className={FEED_GRID_CLASS} aria-label="Reels de la comunidad">
      {items.map((item, index) => (
        <FeedCard
          key={item.id}
          item={item}
          rank={showRank && index < RANKED_POSITIONS ? index + 1 : undefined}
        />
      ))}
    </section>
  );
}

/** Loading placeholder mirroring the grid and the card's real geometry. */
export function FeedGridSkeleton() {
  return (
    <div className={FEED_GRID_CLASS} aria-label="Cargando reels" role="status">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="studio-panel flex flex-col overflow-hidden">
          <Skeleton className="aspect-[4/5] w-full rounded-none" />
          <div className="flex flex-col gap-4 p-4">
            <div className="flex flex-col gap-2">
              <Skeleton className="h-5 w-3/4" />
              <Skeleton className="h-3 w-1/3" />
            </div>
            <div className="flex items-center justify-between gap-4 border-t border-border pt-3">
              <Skeleton className="h-8 w-28" />
              <Skeleton className="h-10 w-20" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
