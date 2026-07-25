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
  // The URL that failed, not a boolean: a card keeps its component instance
  // across the Library's 1.5s poll, so a reel whose cover only exists once the
  // render finishes would stay blank forever if one earlier URL had failed.
  const [failedSrc, setFailedSrc] = useState<string | null>(null);

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
    // Record the prop, not `node.src`: the DOM resolves it to an absolute URL
    // and the comparison below is against the relative path the API returns.
    if (node !== null && node.complete && node.naturalWidth === 0) setFailedSrc(src ?? null);
  };

  if (src === undefined || failedSrc === src) return null;

  return (
    // eslint-disable-next-line @next/next/no-img-element -- dynamic same-origin media proxied by the orchestrator
    <img
      key={src}
      ref={check}
      src={src}
      alt=""
      loading={loading}
      decoding="async"
      onError={() => setFailedSrc(src)}
      className={cn('size-full object-cover', className)}
    />
  );
}
