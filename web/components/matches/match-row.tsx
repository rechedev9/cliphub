import Link from 'next/link';
import { MapCover } from '@/components/brand/map-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { ScoreBar } from '@/components/brand/score-bar';
import { StatMono } from '@/components/brand/stat-mono';
import { StatusTag } from '@/components/studio/status-tag';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { Button } from '@/components/ui/button';
import { formatKd, matchDateLabel } from '@/lib/format';
import { matchPlanReady } from '@/lib/match-plays-empty';
import { cn } from '@/lib/utils';
import type { Match } from '@/lib/api/types';
import { isWin, MatchScore, parseScore } from './match-score';
import { MATCH_ROW_MARKER } from './row-parallax';

export type MatchRowProps = {
  match: Match;
  /**
   * The spotlight row (the first in the list): raised panel, accented edge and
   * the notched FORJAR REEL CTA. Other rows get the quiet outline link instead.
   *
   * It deliberately does NOT add `.neon-brackets`: the brackets are square
   * 14×14 corners pinned at -1px, and a `studio-panel` is rounded, so they float
   * off the corner exactly the way the audit flagged on the Library's capture
   * card. design.md also caps a component at one dominant angular treatment —
   * here that is the accented raised edge, and the notch belongs to the CTA.
   */
  featured?: boolean;
  /** Deletes this match (and its artifacts); when set, the row shows a trash button. */
  onDelete?: (jobId: string) => Promise<void>;
  /** Called after a successful delete so the page can re-fetch its lists. */
  onDeleted?: () => void;
};

/**
 * The map still. Uploaded jobs rarely carry a thumbnail, so the frame paints a
 * map-specific plate (palette + silhouette) and only layers a real image when
 * the API supplied one. A URL that never resolves (CSP is `img-src 'self'
 * data: blob:`) unmounts and leaves the plate, not a broken box.
 */
function MatchThumb({ match }: { match: Match }) {
  return (
    <span className="relative hidden aspect-video w-28 shrink-0 self-center overflow-hidden border border-border-subtle bg-surface-0 @[42rem]/content:block">
      <MapCover map={match.map} className="absolute inset-0" />
      <CoverImage src={match.thumbnailUrl} className="absolute inset-0" />
    </span>
  );
}

/**
 * One scoreboard row. The layout is keyed to `@container/content` (the real
 * content column), not to viewport breakpoints: the previous `xl:` grid needed
 * ~858px of track inside a box that is 544px wide at a 1280px viewport, which is
 * why the densest surface in the product overflowed hardest at exactly the
 * breakpoint meant to enable it.
 *
 * Depth is three composited signals and no layout: the plane tilts by at most
 * 1.1° under the cursor, the content lifts 10px toward a deliberately short
 * 640px perspective, and the specular sweep tracks the pointer. All three read
 * `--shell-depth`, so the efficiency profile, an inactive window, reduced motion
 * and forced colours flatten them through the foundation's single gate. The
 * ceiling is tiny on purpose — this is a dense scoreboard and a text baseline
 * that visibly rotates is a reading regression, not depth.
 */
export function MatchRow({ match, featured = false, onDelete, onDeleted }: MatchRowProps) {
  const win = isWin(match.score);
  const { stats } = match;
  const { ours, theirs } = parseScore(match.score);
  const hasScore = ours !== null && theirs !== null;
  const analyzing = !matchPlanReady(match.status);
  // Lead the meta line with the clipped player when known ("<PLAYER> · HACE X"),
  // dropping it cleanly (no stray separator) when it is absent.
  let playsMeta: string | null = null;
  if (!analyzing && match.decentPlays > 0) {
    playsMeta = `${match.decentPlays} ${match.decentPlays === 1 ? 'jugada' : 'jugadas'}`;
  }
  const meta = [match.player, matchDateLabel(match), playsMeta].filter(Boolean).join(' · ');

  return (
    <article
      {...MATCH_ROW_MARKER}
      className={cn(
        'studio-tilt studio-defer-render group/row',
        // A short perspective is what makes 10px of Z legible at all; at the
        // global 900px the same lift resolves to a 1% scale, i.e. nothing.
        '[--perspective:640px] [--sheen-opacity:calc(var(--shell-depth)*0.06)]',
        'hover:[--row-lift:10px] focus-within:[--row-lift:10px]',
      )}
    >
      <div
        className={cn(
          'studio-tilt-plane studio-tilt-sheen studio-panel after:rounded-[inherit]',
          'px-4 py-4 @[34rem]/content:px-5',
          '[--tilt-max:calc(var(--shell-depth)*1.1deg)]',
          'transition-[border-color,box-shadow] duration-(--dur-base) ease-standard',
          featured
            ? 'studio-panel-raised'
            : 'group-hover/row:border-border-strong group-hover/row:shadow-md group-focus-within/row:border-border-strong group-focus-within/row:shadow-md',
        )}
      >
        <div
          className={cn(
            'flex items-stretch gap-4 @[34rem]/content:gap-5',
            '[transform:translateZ(calc(var(--row-lift,0px)*var(--shell-depth)))]',
            'transition-transform duration-(--dur-base) ease-standard',
          )}
        >
          <ScoreBar win={win} className="w-1 shrink-0" />
          <MatchThumb match={match} />

          <div
            className={cn(
              'grid min-w-0 flex-1 items-center gap-x-4 gap-y-3',
              hasScore ? 'grid-cols-[minmax(0,1fr)_auto]' : 'grid-cols-1',
              hasScore
                ? '@[56rem]/content:grid-cols-[minmax(0,1.2fr)_auto_auto_auto]'
                : '@[56rem]/content:grid-cols-[minmax(0,1.2fr)_auto_auto]',
              '@[56rem]/content:gap-x-6',
            )}
          >
            <div className="min-w-0">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <h2 className="truncate font-display text-title font-bold uppercase text-fg-1">{match.map}</h2>
                {analyzing ? <StatusTag tone="warning">Analizando</StatusTag> : null}
              </div>
              <p className="mt-1 truncate font-mono text-meta uppercase tracking-wider text-fg-3">{meta}</p>
            </div>

            {hasScore ? (
              <MatchScore
                score={match.score}
                className="items-end @[56rem]/content:items-start"
              />
            ) : null}

            <div
              className={cn(
                'col-span-full grid grid-cols-5 gap-3 border-y border-border-subtle py-3 @[34rem]/content:gap-5',
                '@[56rem]/content:col-span-1 @[56rem]/content:flex @[56rem]/content:items-center @[56rem]/content:gap-6 @[56rem]/content:border-0 @[56rem]/content:py-0',
              )}
            >
              <StatMono label="K" value={stats.kills} />
              <StatMono label="D" value={stats.deaths} />
              <StatMono label="A" value={stats.assists} />
              <StatMono label="MVP" value={stats.mvps} />
              <StatMono label="K/D" value={formatKd(stats.kd)} accent />
            </div>

            <div className="col-span-full flex min-w-0 items-center justify-end gap-2 @[56rem]/content:col-span-1">
              <MatchRowCta matchId={match.id} analyzing={analyzing} featured={featured} />
              {onDelete ? (
                <DeleteMatchButton
                  label={match.map}
                  onConfirm={() => onDelete(match.id)}
                  onDeleted={() => onDeleted?.()}
                />
              ) : null}
            </div>
          </div>
        </div>
      </div>
    </article>
  );
}

function MatchRowCta({
  matchId,
  analyzing,
  featured,
}: {
  matchId: string;
  analyzing: boolean;
  featured: boolean;
}) {
  if (analyzing) {
    return (
      <Button
        asChild
        variant="outline"
        className="flex-1 font-mono text-meta uppercase tracking-wider text-fg-3 @[34rem]/content:flex-initial"
      >
        <Link href={`/matches/${matchId}`}>VER ESTADO</Link>
      </Button>
    );
  }
  if (featured) {
    return (
      <Button
        asChild
        variant="hero"
        className="neon-notch flex-1 rounded-none @[34rem]/content:flex-initial"
      >
        <Link href={`/matches/${matchId}`}>FORJAR REEL</Link>
      </Button>
    );
  }
  return (
    <Button
      asChild
      variant="outline"
      className="flex-1 font-mono text-meta uppercase tracking-wider text-fg-2 @[34rem]/content:flex-initial"
    >
      <Link href={`/matches/${matchId}`}>VER PARTIDA</Link>
    </Button>
  );
}
