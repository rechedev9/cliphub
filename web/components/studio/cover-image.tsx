'use client';

import { useState, type ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type CoverImageProps = {
  /** Cover URL from the API. Undefined renders nothing at all. */
  src: string | undefined;
  className?: string;
  loading?: 'lazy' | 'eager';
};

/**
 * A cover image that disappears instead of failing loudly.
 *
 * Every media surface in Studio paints a seeded `ReelCover` plate underneath the
 * real thumbnail so an unloadable URL degrades to brand art. That alone is not
 * enough: a broken `<img>` still paints the browser's broken-image glyph on top
 * of the plate, which is exactly what the highlight selector, the match rows and
 * the reel cards were all showing. Unmounting on the first `error` is what makes
 * the fallback actually visible.
 *
 * This matters here more than in a typical app because the CSP is
 * `img-src 'self' data: blob:` — any off-origin cover the orchestrator reports
 * is guaranteed to fail, not merely likely to.
 */
export function CoverImage({ src, className, loading = 'lazy' }: CoverImageProps): ReactNode {
  const [failed, setFailed] = useState(false);

  /*
   * `onError` alone is not enough. The browser starts fetching as soon as it
   * parses the tag, so a cover that fails during hydration has already fired its
   * error event by the time React attaches the handler — and React does not
   * replay it. The ref callback re-checks the settled state on mount:
   * `complete && naturalWidth === 0` is the DOM's way of saying "this one is
   * already broken", which is precisely the case that left a stretched
   * broken-image ring across every Library card.
   */
  const check = (node: HTMLImageElement | null): void => {
    if (node !== null && node.complete && node.naturalWidth === 0) setFailed(true);
  };

  if (src === undefined || failed) return null;

  return (
    // eslint-disable-next-line @next/next/no-img-element -- dynamic same-origin media proxied by the orchestrator
    <img
      ref={check}
      src={src}
      alt=""
      loading={loading}
      decoding="async"
      onError={() => setFailed(true)}
      className={cn('size-full object-cover', className)}
    />
  );
}
