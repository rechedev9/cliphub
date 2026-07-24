import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type MediaFrameAspect = '16:9' | '9:16' | '1:1';

const ASPECT_CLASS = {
  '16:9': 'aspect-video',
  '9:16': 'aspect-[9/16]',
  '1:1': 'aspect-square',
} as const satisfies Record<MediaFrameAspect, string>;

/**
 * Grid cap for a portrait frame. At a 260px library column a true 9:16 frame is
 * 462px tall and the card lands past 750px — taller than half the workspace, and
 * it strands the row's shorter siblings in dead space. Capping the height keeps
 * the frame unambiguously portrait (still ~1.35:1) while the format badge states
 * the exact shape, so nothing is misrepresented.
 */
const CAPPED_ASPECT_CLASS = {
  '16:9': 'aspect-video',
  '9:16': 'aspect-[9/16] max-h-[22rem]',
  '1:1': 'aspect-square',
} as const satisfies Record<MediaFrameAspect, string>;

export type MediaFrameProps = {
  /**
   * The reel's real shape. A shorts tool whose library crops every 9:16 reel to
   * 16:9 is showing the user something it did not make, so callers drive this
   * from `editConfig.format` rather than accepting the default.
   */
  aspect?: MediaFrameAspect;
  /**
   * Cap a portrait frame's height so a mixed grid stays scannable. Off for the
   * places that must show the true shape at full size (a player, a preview).
   */
  capHeight?: boolean;
  /** The media itself. A direct `img`/`video` child is covered automatically. */
  media?: ReactNode;
  /** Painted when `media` is null or undefined — e.g. a generated reel cover. */
  fallback?: ReactNode;
  /** Top-right corner slot: format, duration, kill count. */
  badge?: ReactNode;
  /** Bottom-left corner slot: stage label, map, REC state. */
  footer?: ReactNode;
  /** Actions revealed on hover — always keyboard-reachable, always on for touch. */
  actions?: ReactNode;
  /** Bottom-up gradient so overlaid text keeps its contrast against the media. */
  scrim?: boolean;
  /** Fine CRT pitch overlay. */
  scanline?: boolean;
  className?: string;
};

/**
 * The shared media box: aspect, cover fallback, corner slots, scrim and a hover
 * action layer. The action layer fades with opacity rather than unmounting, so
 * the buttons stay in the tab order and `group-focus-within` reveals them for
 * keyboard users; `(hover: none)` pins them visible because a touch device has
 * no hover to reveal them with (design.md: never hide essential controls behind
 * hover only).
 */
export function MediaFrame({
  aspect = '16:9',
  capHeight = false,
  media,
  fallback,
  badge,
  footer,
  actions,
  scrim = false,
  scanline = false,
  className,
}: MediaFrameProps): ReactNode {
  return (
    <div
      className={cn(
        'group/frame relative w-full overflow-hidden bg-surface-0',
        capHeight ? CAPPED_ASPECT_CLASS[aspect] : ASPECT_CLASS[aspect],
        scanline && 'studio-scanline',
        className,
      )}
    >
      <div className="absolute inset-0 [&>img]:size-full [&>img]:object-cover [&>video]:size-full [&>video]:object-cover">
        {media ?? fallback}
      </div>

      {scrim ? (
        <span
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-gradient-to-t from-surface-0/80 via-transparent to-surface-0/15"
        />
      ) : null}

      {badge ? <div className="absolute top-2.5 right-2.5 flex items-center gap-1.5">{badge}</div> : null}
      {footer ? <div className="absolute bottom-2.5 left-2.5 flex items-center gap-1.5">{footer}</div> : null}

      {actions ? (
        <div
          className={cn(
            'pointer-events-none absolute inset-0 flex items-center justify-center gap-2 bg-surface-0/80 opacity-0',
            'transition-opacity duration-(--dur-base) ease-standard',
            'group-hover/frame:pointer-events-auto group-hover/frame:opacity-100',
            'group-focus-within/frame:pointer-events-auto group-focus-within/frame:opacity-100',
            '[@media(hover:none)]:pointer-events-auto [@media(hover:none)]:opacity-100',
          )}
        >
          {actions}
        </div>
      ) : null}
    </div>
  );
}
