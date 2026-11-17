'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { CoverImage } from '@/components/studio/cover-image';

// Past a Short's first cut and a Full Demo's fade-in, so the frame shows gameplay.
const FRAME_SECONDS = 3;

export type VideoPosterProps = {
  posterUrl: string | undefined;
  /** A Range-capable MP4 URL; its frame stands in when the render has no cover image. */
  videoUrl: string | undefined;
  fallback: ReactNode;
  loading?: 'lazy' | 'eager';
};

/**
 * The render's cover, else one frame of the MP4 itself, else the brand plate.
 * The `<video>` mounts only near the viewport: each one holds a connection while it reads the metadata.
 */
export function VideoPoster({ posterUrl, videoUrl, fallback, loading = 'lazy' }: VideoPosterProps): ReactNode {
  const holder = useRef<HTMLSpanElement>(null);
  const [visible, setVisible] = useState(loading === 'eager');
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  const wantsFrame = posterUrl === undefined && videoUrl !== undefined && failedUrl !== videoUrl;

  useEffect(() => {
    const node = holder.current;
    if (!wantsFrame || visible || node === null) return;
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) setVisible(true);
    }, { rootMargin: '200px' });
    observer.observe(node);
    return () => observer.disconnect();
  }, [visible, wantsFrame]);

  if (posterUrl !== undefined) return <CoverImage src={posterUrl} loading={loading} />;
  if (!wantsFrame || videoUrl === undefined) return fallback;
  return (
    <span ref={holder} className="relative block size-full">
      <span className="absolute inset-0">{fallback}</span>
      {visible ? (
        <video
          key={videoUrl}
          src={`${videoUrl}#t=${FRAME_SECONDS}`}
          preload="metadata"
          muted
          playsInline
          disablePictureInPicture
          tabIndex={-1}
          aria-hidden
          className="pointer-events-none absolute inset-0 size-full object-cover"
          onError={() => setFailedUrl(videoUrl)}
        />
      ) : null}
    </span>
  );
}
