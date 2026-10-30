import { FLOW_STEPS } from './copy.ts';
import { STREAM_WORKFLOW_STEPS } from '../streams/editor.ts';

/** Every Crear flow reads as the same three phases; each keeps its own step names inside them. */
export const CREATE_PHASES = ['Fuente', 'Ajustes', 'Resultado'] as const;

export const CREATE_FLOW = { short: 'short', full: 'full', stream: 'stream' } as const;
export type CreateFlow = (typeof CREATE_FLOW)[keyof typeof CREATE_FLOW];

export const CREATE_FLOW_STEPS: Record<CreateFlow, readonly string[]> = {
  short: FLOW_STEPS.short,
  full: FLOW_STEPS.full,
  stream: STREAM_WORKFLOW_STEPS,
};

/** How many of a flow's own steps fall in source, settings and result, in that order. */
const PHASE_SIZES: Record<CreateFlow, readonly [number, number, number]> = {
  short: [2, 1, 1],
  full: [2, 1, 1],
  stream: [1, 3, 1],
};

export type CreatePhaseState = 'done' | 'current' | 'pending';
export type CreatePhase = {
  readonly label: (typeof CREATE_PHASES)[number];
  readonly steps: readonly string[];
  readonly state: CreatePhaseState;
  /** The step on screen when this phase is current, else the phase's first step. */
  readonly detail: string;
};

/** The three phases of a flow with `step` (an index into its own steps) placed inside them. */
export function createFlowPhases(flow: CreateFlow, step: number): CreatePhase[] {
  const steps = CREATE_FLOW_STEPS[flow];
  const current = Math.min(Math.max(0, step), steps.length - 1);
  let start = 0;
  return CREATE_PHASES.map((label, index) => {
    const size = PHASE_SIZES[flow][index] ?? 0;
    const own = steps.slice(start, start + size);
    let state: CreatePhaseState = 'pending';
    if (current >= start + size) state = 'done';
    else if (current >= start) state = 'current';
    const detail = state === 'current' ? (steps[current] ?? '') : (own[0] ?? '');
    start += size;
    return { label, steps: own, state, detail };
  });
}
