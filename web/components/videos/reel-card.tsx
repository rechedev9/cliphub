import type { ReactNode } from 'react';
import type { EditConfig, RenderFormat, Video } from '@/lib/api/types';
import { prettyMapName, timeAgo } from '@/lib/format';
import { cn } from '@/lib/utils';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { MediaFrame, type MediaFrameAspect } from '@/components/studio/media-frame';
import { TiltSurface } from '@/components/studio/tilt-surface';
import { ReelStageTrack } from '@/components/videos/reel-stage-track';

/** Frame aspect follows the render format, not a hardcoded landscape box. */
const FORMAT_ASPECT = {
  'short-9x16': '9:16',
  'landscape-16x9': '16:9',
} as const satisfies Record<RenderFormat, MediaFrameAspect>;

const FORMAT_LABEL = {
  'short-9x16': '9:16',
  'landscape-16x9': '16:9',
} as const satisfies Record<RenderFormat, string>;

/** Neutral box when `editConfig` is missing. Label stays undefined for the same input. */
const UNKNOWN_FORMAT_ASPECT: MediaFrameAspect = '16:9';

export function reelAspect(config: EditConfig | undefined): MediaFrameAspect {
  return config === undefined ? UNKNOWN_FORMAT_ASPECT : FORMAT_ASPECT[config.format];
}

export function reelFormatLabel(config: EditConfig | undefined): string | undefined {
  return config === undefined ? undefined : FORMAT_LABEL[config.format];
}

export type ReelCardTone = 'neutral' | 'primary' | 'stream' | 'danger';

/** Utility layer wins over `.studio-panel-raised` border-color. */
const TONE_BORDER_CLASS = {
  neutral: 'border-border',
  primary: 'border-primary/45',
  stream: 'border-stream/45',
  danger: 'border-destructive/45',
} as const satisfies Record<ReelCardTone, string>;

export type ReelCardProps = {
  video: Video;
  /** Edge accent. Carries the pipeline stage, never on its own — tags do too. */
  tone?: ReelCardTone;
  /** A step up the surface ramp plus real elevation, for the payoff state. */
  raised?: boolean;
  /** Top-right slot of the frame: the format tag. */
  badge?: ReactNode;
  /** Bottom-left slot of the frame: the stage indicator. */
  frameFooter?: ReactNode;
  /** Revealed on hover, on focus-within and always on touch. */
  frameActions?: ReactNode;
  /** Dim/desaturate the cover while the reel has no finished frame yet. */
  coverClassName?: string;
  /** Hide the fallback-plate crosshair so processing is not read as a spinner. */
  plainCover?: boolean;
  /** Stage tint painted over the cover, inside the parallax plane. */
  coverTintClassName?: string;
  /** Bottom-up gradient, for frames that carry overlaid text. */
  scrim?: boolean;
  /** Real capture progress, 0-100, fed to the running stage segment. */
  percent?: number;
  /** Data block under title + meta. */
  children?: ReactNode;
  /** Action block under the stage track — the card's bottom edge. */
  footer?: ReactNode;
};

/** Shared reel shell. Tilt lives on the overscaled cover; `--shell-depth` flattens it. */
export function ReelCard({
  video,
  tone = 'neutral',
  raised = false,
  badge,
  frameFooter,
  frameActions,
  coverClassName,
  plainCover = false,
  coverTintClassName,
  scrim = false,
  percent,
  children,
  footer,
}: ReelCardProps): ReactNode {
  const map = prettyMapName(video.map);
  const matchMeta = video.score ? `${map} · ${video.score}` : map;
  const meta = video.targetName ? `POV ${video.targetName} · ${matchMeta}` : matchMeta;

  /* Plate under CoverImage: a failed <img> would show Chrome's broken-image glyph. */
  const cover = (
    <>
      <ReelCover
        seed={video.id}
        plain={plainCover}
        className={cn('absolute inset-0 size-full', coverClassName)}
      />
      <CoverImage src={video.thumbnailUrl} className={cn('absolute inset-0', coverClassName)} />
    </>
  );

  return (
    <article
      data-slot="card"
      className={cn(
        'studio-panel studio-panel-interactive studio-defer-render studio-reveal flex flex-col overflow-hidden',
        raised && 'studio-panel-raised',
        TONE_BORDER_CLASS[tone],
      )}
    >
      <MediaFrame
        aspect={reelAspect(video.editConfig)}
        capHeight
        className="studio-rim border-b border-border"
        scrim={scrim}
        scanline
        badge={badge}
        footer={frameFooter}
        actions={frameActions}
        media={
          <TiltSurface className="size-full" planeClassName="size-full">
            {/* Overscaled: a 6° rotation pulls one edge ~2px inside the frame,
                so the plane's content has to be wider than the box it fills or
                the parallax would expose the panel behind it. */}
            <div className="relative size-full scale-[1.06] transition-transform duration-(--dur-base) ease-standard group-hover/frame:scale-[1.1]">
              {cover}
              {coverTintClassName !== undefined ? (
                <span aria-hidden className={cn('absolute inset-0', coverTintClassName)} />
              ) : null}
            </div>
          </TiltSurface>
        }
      />

      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="min-w-0">
          <p title={video.title} className="truncate font-display text-body-lg font-bold text-fg-1">
            {video.title}
          </p>
          <div className="mt-1.5 flex items-baseline justify-between gap-2 font-mono text-meta uppercase tabular-nums text-fg-3">
            <span className="min-w-0 truncate">{meta}</span>
            <span className="shrink-0">{timeAgo(video.createdAt)}</span>
          </div>
        </div>
        {children}
      </div>

      <ReelStageTrack status={video.status} percent={percent} />

      {footer}
    </article>
  );
}
