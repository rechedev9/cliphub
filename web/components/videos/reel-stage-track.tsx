import { AlertTriangle } from 'lucide-react';
import type { ReactNode } from 'react';
import type { VideoStatus } from '@/lib/api/types';
import { cn } from '@/lib/utils';

/**
 * The four product-facing pipeline stages, in the order the rig runs them. The
 * labels are written in sentence case and uppercased in CSS so a screen reader
 * announces words rather than initialisms.
 */
const STAGES = [
  { status: 'queued', label: 'Cola' },
  { status: 'recording', label: 'Captura' },
  { status: 'composing', label: 'Edición' },
  { status: 'ready', label: 'Listo' },
] as const satisfies readonly { status: Exclude<VideoStatus, 'failed'>; label: string }[];

/** CAPTURA is the REC stage, so its accent is magenta rather than cyan. */
const CAPTURE_INDEX = 1;
const FINAL_INDEX = STAGES.length - 1;

const STATE_WORD = {
  done: 'completado',
  active: 'en curso',
  todo: 'pendiente',
} as const;

type SegmentState = keyof typeof STATE_WORD;
type SegmentAccent = 'primary' | 'stream' | 'success';

/** The lit bar along the top edge of the running (or finished) segment. */
const RIM_CLASS = {
  primary: 'bg-primary shadow-[var(--glow-primary-sm)]',
  stream: 'bg-stream shadow-[var(--glow-stream-md)]',
  // No --glow-success token exists, and a literal blur would not inherit the
  // efficiency downgrade the two above get for free. The rim carries it alone.
  success: 'bg-success',
} as const satisfies Record<SegmentAccent, string>;

/** The meter fill behind the label. Alpha only: the label sits on top of it. */
const FILL_CLASS = {
  primary: 'bg-primary/20',
  stream: 'bg-stream/22',
  success: 'bg-success/20',
} as const satisfies Record<SegmentAccent, string>;

const TEXT_CLASS = {
  primary: 'text-primary',
  stream: 'text-stream-text',
  success: 'text-success',
} as const satisfies Record<SegmentAccent, string>;

/**
 * Same three shell signals `Skeleton` and `Progress` name. The depth gate can
 * flatten a transform but cannot stop an animation, and an indeterminate sweep
 * on every queued reel in the Library would keep the compositor awake while
 * cs2.exe wants the GPU.
 */
const INDETERMINATE_MOTION_GATE =
  "[html[data-performance-profile='efficiency']_&]:animate-none [html[data-window-activity='inactive']_&]:animate-none [html[data-capture-active='true']_&]:animate-none";

function segmentState(index: number, active: number, finished: boolean): SegmentState {
  if (index > active) return 'todo';
  if (index < active) return 'done';
  return finished ? 'done' : 'active';
}

function segmentAccent(index: number, finished: boolean): SegmentAccent {
  if (finished && index === FINAL_INDEX) return 'success';
  return index === CAPTURE_INDEX ? 'stream' : 'primary';
}

/**
 * Text tone. The running segment and the finished LISTO keep their accent — the
 * payoff stage stays lit rather than dimming with the rest of the ladder — and
 * every other step falls back to the foreground ramp so the strip reads as data
 * and not as four coloured chips.
 */
function segmentTextClass(index: number, state: SegmentState, finished: boolean): string {
  if (state === 'active') return TEXT_CLASS[segmentAccent(index, finished)];
  if (finished && index === FINAL_INDEX) return TEXT_CLASS.success;
  return state === 'done' ? 'text-fg-2' : 'text-fg-3';
}

export type ReelStageTrackProps = {
  status: VideoStatus;
  /**
   * Real capture progress, 0-100, for the stage that is running. Omit it when
   * the orchestrator reports none: the segment then sweeps instead of showing a
   * percentage the pipeline never produced.
   */
  percent?: number;
  className?: string;
};

/**
 * The reel card's instrument strip: one full-bleed element carrying BOTH the
 * pipeline stage and its progress, where the card used to stack a text step list
 * on top of a separate bar.
 *
 * Depth comes from the surface ramp, not from a shadow — the strip is a
 * `--surface-0` well recessed into the panel, the running segment steps back up
 * to `--surface-2`, and a 2px lit rim marks it. All three survive the efficiency
 * profile, unlike a blurred glow, because two of them are colours.
 */
export function ReelStageTrack({ status, percent, className }: ReelStageTrackProps): ReactNode {
  if (status === 'failed') {
    // A generic failure carries no reliable failed-stage data, so the strip
    // states the fact instead of inventing a position on the ladder.
    return (
      <p
        className={cn(
          'flex min-h-9 items-center gap-2 border-t border-destructive/45 bg-destructive/12 px-3',
          'font-mono text-meta uppercase text-destructive',
          className,
        )}
      >
        <AlertTriangle aria-hidden className="size-3.5 shrink-0" />
        Error de pipeline
      </p>
    );
  }

  const active = STAGES.findIndex((stage) => stage.status === status);
  const finished = status === 'ready';
  const pct = percent === undefined ? undefined : Math.min(100, Math.max(0, Math.round(percent)));

  return (
    <ol
      aria-label="Progreso del reel"
      className={cn('grid grid-cols-4 border-t border-border bg-surface-0', className)}
    >
      {STAGES.map((stage, index) => {
        const state = segmentState(index, active, finished);
        const accent = segmentAccent(index, finished);
        const running = state === 'active';
        const determinate = running && pct !== undefined;

        return (
          <li
            key={stage.status}
            data-state={state}
            aria-current={running ? 'step' : undefined}
            className={cn(
              // px-1, not px-2: at the narrowest card the grid allows, a
              // 4-segment strip leaves ~61px per segment and "CAPTURA" measures
              // 50px at `text-meta` + `tracking-wide`. Every looser combination
              // truncated the two longest stage names.
              'relative isolate flex min-h-9 items-center justify-center overflow-hidden px-1',
              index > 0 && 'border-l border-border-subtle',
              running && 'bg-surface-2',
            )}
          >
            {state === 'done' ? (
              <span aria-hidden className={cn('absolute inset-0 -z-10', FILL_CLASS[accent])} />
            ) : null}

            {determinate ? (
              <span
                role="progressbar"
                aria-label={`${stage.label}, progreso`}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={pct}
                style={{ width: `${pct}%` }}
                className={cn(
                  'absolute inset-y-0 left-0 -z-10 transition-[width] duration-(--dur-data) ease-standard',
                  FILL_CLASS[accent],
                )}
              />
            ) : null}

            {running && !determinate ? (
              <span
                aria-hidden
                className={cn('absolute inset-0 -z-10 animate-pulse', FILL_CLASS[accent], INDETERMINATE_MOTION_GATE)}
              />
            ) : null}

            {running || (finished && index === FINAL_INDEX) ? (
              <span aria-hidden className={cn('absolute inset-x-0 top-0 h-0.5', RIM_CLASS[accent])} />
            ) : null}

            <span
              className={cn(
                'min-w-0 truncate font-mono text-meta tracking-wide uppercase',
                segmentTextClass(index, state, finished),
              )}
            >
              {stage.label}
            </span>
            <span className="sr-only">{`, ${STATE_WORD[state]}`}</span>
          </li>
        );
      })}
    </ol>
  );
}
