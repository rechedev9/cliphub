export const CREATE_FLOW = { short: 'short', full: 'full', stream: 'stream' } as const;
export type CreateFlow = (typeof CREATE_FLOW)[keyof typeof CREATE_FLOW];

/** The steps the rail names, short enough to sit side by side. A demo flow ends by recording the video. */
export const CREATE_FLOW_STEPS: Record<CreateFlow, readonly string[]> = {
  short: ['Demo', 'Jugador', 'Jugadas', 'Grabar'],
  full: ['Demo', 'Jugador', 'Ajustes', 'Grabar'],
  stream: ['Vídeo', 'Momentos', 'Aspecto', 'Revisar', 'Guardar'],
};

export type CreateStepState = 'done' | 'current' | 'pending';
export type CreateStep = { readonly label: string; readonly state: CreateStepState };

/** The steps of a flow with `step` (an index into them) marked current; an out-of-range step is clamped. */
export function createFlowSteps(flow: CreateFlow, step: number): CreateStep[] {
  const steps = CREATE_FLOW_STEPS[flow];
  const current = Math.min(Math.max(0, step), steps.length - 1);
  return steps.map((label, index) => {
    let state: CreateStepState = 'pending';
    if (index < current) state = 'done';
    else if (index === current) state = 'current';
    return { label, state };
  });
}
