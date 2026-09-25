import { isFullDemoBumperOptions, type FullDemoAssetRef, type FullDemoBumperOptions, type FullDemoOptions } from '../full-demo-plan.ts';

/**
 * The intro, sponsor and outro clips last chosen in any Full Demo, kept in the
 * browser so the next demo starts with them. The clips live in the global
 * editor asset library, so a reference chosen for one match is valid for any
 * other one.
 */
export const FULL_DEMO_BUMPER_MEMORY_KEY = 'cliphub.full-demo.bumpers.v1';

const OFF = { enabled: false, video: null } as const;

/** Only slots with a clip are kept; the sponsor slot stays absent without one, like Go's omitempty. */
export function rememberedBumpers(bumpers: FullDemoBumperOptions | undefined): FullDemoBumperOptions | undefined {
  const pick = (slot: FullDemoBumperOptions['intro'] | undefined) => slot?.enabled && slot.video ? { enabled: true, video: slot.video } : null;
  const intro = pick(bumpers?.intro), sponsor = pick(bumpers?.sponsor), outro = pick(bumpers?.outro);
  if (!intro && !sponsor && !outro) return undefined;
  return { intro: intro ?? OFF, outro: outro ?? OFF, ...(sponsor ? { sponsor } : {}) };
}

export function rememberFullDemoBumpers(bumpers: FullDemoBumperOptions | undefined, storage: Storage = localStorage): void {
  const kept = rememberedBumpers(bumpers);
  try {
    if (kept) storage.setItem(FULL_DEMO_BUMPER_MEMORY_KEY, JSON.stringify(kept));
    else storage.removeItem(FULL_DEMO_BUMPER_MEMORY_KEY);
  } catch { }
}

export function recallFullDemoBumpers(storage: Storage = localStorage): FullDemoBumperOptions | undefined {
  try {
    const value: unknown = JSON.parse(storage.getItem(FULL_DEMO_BUMPER_MEMORY_KEY) ?? 'null');
    return isFullDemoBumperOptions(value) ? rememberedBumpers(value) : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Drops remembered clips that were deleted from the library or replaced by
 * different bytes, so a fresh demo never opens with a clip the planner would
 * block. A clip whose check fails for any other reason is kept: the planner
 * still verifies it.
 */
export async function availableFullDemoBumpers(bumpers: FullDemoBumperOptions | undefined, signal?: AbortSignal, fetcher: typeof fetch = fetch): Promise<FullDemoBumperOptions | undefined> {
  if (!bumpers) return undefined;
  const available = async (video: FullDemoAssetRef | null): Promise<boolean> => {
    if (!video) return false;
    try {
      const response = await fetcher(`/api/editor/assets/${video.id}`, { signal });
      if (response.status === 404) return false;
      if (!response.ok) return true;
      const value: unknown = await response.json();
      return !(typeof value === 'object' && value !== null && 'sha256' in value && typeof value.sha256 === 'string' && value.sha256 !== video.sha256);
    } catch {
      return !signal?.aborted;
    }
  };
  const [intro, sponsor, outro] = await Promise.all([bumpers.intro, bumpers.sponsor, bumpers.outro].map((slot) => available(slot?.video ?? null)));
  return rememberedBumpers({
    intro: intro ? bumpers.intro : OFF,
    outro: outro ? bumpers.outro : OFF,
    ...(sponsor && bumpers.sponsor ? { sponsor: bumpers.sponsor } : {}),
  });
}

/** Server defaults for a demo that was never planned, with the remembered clips loaded. */
export function withRememberedBumpers(defaults: FullDemoOptions, bumpers: FullDemoBumperOptions | undefined): FullDemoOptions {
  return bumpers && !rememberedBumpers(defaults.bumpers) ? { ...defaults, bumpers } : defaults;
}
