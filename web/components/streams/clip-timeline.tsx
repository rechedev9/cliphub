import type { ReactNode } from 'react';
import type { StreamClipRange } from '@/lib/api/streams';
import {
  clipOutputDuration,
  clipSourceDuration,
  clipTimelineGeometry,
  formatStreamTimestamp,
  overlayMarkerGeometry,
} from '@/lib/streams/plan';

/**
 * The clip drawn on the source timeline: where it starts, how much of the VOD
 * it takes, where its fades bite and where each burned-in text sits.
 *
 * This is the piece that turns nine number inputs into an edit. It renders only
 * from values already in the plan — no probe duration means no scale, so the
 * strip is omitted rather than drawn against an invented length.
 */
export function StreamClipTimeline({
  clip,
  sourceDuration,
}: {
  clip: StreamClipRange;
  sourceDuration: number;
}): ReactNode {
  const geometry = clipTimelineGeometry(clip, sourceDuration);
  if (geometry === null) return null;

  const span = clipSourceDuration(clip);
  const overlays = clip.edit?.text_overlays ?? [];
  const speed = clip.edit?.speed ?? 1;

  return (
    <div className="flex flex-col gap-2">
      <div
        aria-hidden
        className="studio-rim relative h-10 w-full overflow-hidden border border-border-strong bg-surface-0"
      >
        <span
          className="absolute inset-y-0 border-x-2 border-stream bg-stream/25"
          style={{ left: `${geometry.startPercent}%`, width: `${geometry.widthPercent}%` }}
        >
          {geometry.fadeInPercent > 0 ? (
            <span
              className="absolute inset-y-0 left-0 bg-gradient-to-r from-surface-0 to-transparent"
              style={{ width: `${geometry.fadeInPercent}%` }}
            />
          ) : null}
          {geometry.fadeOutPercent > 0 ? (
            <span
              className="absolute inset-y-0 right-0 bg-gradient-to-l from-surface-0 to-transparent"
              style={{ width: `${geometry.fadeOutPercent}%` }}
            />
          ) : null}
          {overlays.map((overlay, index) => {
            const marker = overlayMarkerGeometry(overlay, span);
            if (marker === null) return null;
            return (
              <span
                key={index}
                className="absolute bottom-0 h-2 bg-primary shadow-[var(--glow-primary-sm)]"
                style={{ left: `${marker.startPercent}%`, width: `${marker.widthPercent}%` }}
              />
            );
          })}
        </span>
      </div>

      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 font-mono text-meta tabular-nums">
        <span className="text-stream-text">
          {formatStreamTimestamp(clip.start_seconds)} → {formatStreamTimestamp(clip.end_seconds)}
          <span className="text-fg-3"> · {span.toFixed(2)} s</span>
        </span>
        <span className="text-fg-3">
          {speed === 1 ? null : <span className="text-warning">{speed}× · </span>}
          SALIDA {clipOutputDuration(clip).toFixed(2)} S · FUENTE {formatStreamTimestamp(sourceDuration)}
        </span>
      </div>
    </div>
  );
}
