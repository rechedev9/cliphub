import { cn } from '@/lib/utils';
import type { VideoStatus } from '@/lib/api/types';

type PipelineStatus = Exclude<VideoStatus, 'failed' | 'review_required'>;

export type PipelineStepsProps = {
  status: VideoStatus;
  className?: string;
};

/** The four product-facing pipeline stages, in order (visible labels). */
const STEPS = ['COLA', 'CAPTURA', 'EDICIÓN', 'LISTO'] as const;

/** CAPTURA is the REC stage, so its rim light is magenta instead of cyan. */
const CAPTURE_STEP = 1;

/**
 * Depth-ladder state, mirrored verbatim into `data-state`: `.pipeline-rail`
 * keys each plate's --z, opacity and rim light off these exact three values.
 */
const STEP_STATE = {
  done: 'done',
  active: 'active',
  todo: 'todo',
} as const;

type StepState = (typeof STEP_STATE)[keyof typeof STEP_STATE];

/**
 * The spoken state. Depth is a weaker channel than colour and design.md forbids
 * relying on colour alone, so every plate carries its state as real text.
 */
const STATE_WORD: Record<StepState, string> = {
  done: 'completado',
  active: 'en curso',
  todo: 'pendiente',
};

/**
 * Pending plates recede further the further out they are, so the row reads as a
 * ladder rather than a list. Written as literal classes because Tailwind only
 * emits the arbitrary properties it can see in the source; the first value is
 * `.pipeline-rail`'s own todo step, so this extends the ladder instead of
 * contradicting it.
 */
const TODO_DEPTH = ['[--z:-20]', '[--z:-28]', '[--z:-36]'] as const;

/**
 * The running plate is pulled back from `.pipeline-rail`'s own +28: the rail
 * translates each plate along its *rotated* Z axis, so a forward plate also
 * slides left, and at +28 it lands 14px over the label before it. +16 keeps a
 * clearly forward step while the slide stays inside the neighbour's padding.
 */
const ACTIVE_DEPTH = '[--z:16]';

/**
 * Separator between plates. A pseudo-element and not a `border-l`, because at
 * the 12px floor the four stage labels need every pixel: three 1px borders are
 * the difference between one line and a 3+1 wrap inside a 300px reel card.
 */
const DIVIDER_CLASS = 'before:absolute before:inset-y-1 before:left-0 before:w-px before:bg-border-strong';

/**
 * Widths are tight on purpose. The rail measures 264px at `--text-meta`, and a
 * 300px reel card leaves 266px inside `studio-panel` + `p-4`; every looser
 * combination of padding, gutter and border wrapped LISTO onto its own line.
 * Below ~290px it wraps regardless — the four stage names simply need that many
 * pixels once 12px is the floor.
 */
const RAIL_CLASS = 'pipeline-rail flex flex-wrap gap-x-1 gap-y-1 font-mono uppercase';

/**
 * Index of the stage a status sits at:
 * queued→COLA, recording→CAPTURA, composing→EDICIÓN, ready→LISTO.
 * A generic failed status is intentionally not mapped because it carries no
 * reliable failed-stage data.
 */
function activeIndex(status: PipelineStatus): number {
  switch (status) {
    case 'queued':
      return 0;
    case 'recording':
      return 1;
    case 'composing':
      return 2;
    case 'ready':
      return 3;
  }
}

function stepState(index: number, active: number, finished: boolean): StepState {
  if (index > active) return STEP_STATE.todo;
  if (index < active) return STEP_STATE.done;
  return finished ? STEP_STATE.done : STEP_STATE.active;
}

/**
 * Text tone. `todo` reaches for the *brightest* ramp step on purpose: the rail
 * composites pending plates at opacity .55, which drops --fg-2 to 3.75:1 (the
 * exact AA failure this component used to ship as `text-muted-foreground/60`)
 * and --fg-1 to 5.77:1. `done` composites at .85, where --fg-2 lands at 7.34:1.
 */
function stepTone(index: number, state: StepState, finished: boolean): string {
  if (state === STEP_STATE.active) {
    return index === CAPTURE_STEP ? 'text-stream-text' : 'text-primary';
  }
  // The finished LISTO keeps the cyan payoff instead of dimming with the rest.
  if (finished && index === STEPS.length - 1) return 'text-primary';
  return state === STEP_STATE.done ? 'text-fg-2' : 'text-fg-1';
}

/**
 * PipelineSteps — the hero of the product story, rendered as a 3D depth ladder:
 * COLA · CAPTURA · EDICIÓN · LISTO as segmented plates on `.pipeline-rail`, with
 * completed stages sitting just behind the plane, the running stage forward
 * under an accent rim light (magenta while capturing — the REC colour — cyan
 * otherwise), and pending stages stacked furthest back. Generic failures render
 * no tracker rather than inventing a failed stage.
 *
 * The rail's arrangement is a static transform, so it survives reduced motion
 * and the efficiency profile; only its transition degrades.
 */
export function PipelineSteps({ status, className }: PipelineStepsProps) {
  if (status === 'failed') return null;

  const pipelineStatus = status === 'review_required' ? 'ready' : status;
  const active = activeIndex(pipelineStatus);
  const finished = pipelineStatus === 'ready';

  return (
    <ol aria-label="Progreso del reel" className={cn(RAIL_CLASS, className)}>
      {STEPS.map((label, index) => {
        const state = stepState(index, active, finished);

        return (
          <li
            key={label}
            data-state={state}
            aria-current={index === active ? 'step' : undefined}
            className={cn(
              'relative px-2 py-1.5',
              index > 0 && DIVIDER_CLASS,
              index === CAPTURE_STEP && '[--pipeline-accent:var(--stream)]',
              state === STEP_STATE.active && ACTIVE_DEPTH,
              state === STEP_STATE.todo && TODO_DEPTH[Math.min(index - active - 1, TODO_DEPTH.length - 1)],
            )}
          >
            <span className={cn('block text-meta', stepTone(index, state, finished))}>{label}</span>
            <span className="sr-only">{`, ${STATE_WORD[state]}`}</span>
          </li>
        );
      })}
    </ol>
  );
}
