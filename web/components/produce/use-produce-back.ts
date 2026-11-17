'use client';

import { useCallback, useState } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { getLibraryJobDemo } from '@/lib/api/library';
import { flowHref, seriesHref, type ProduceFormat } from '@/lib/clips/routes';
import { crearHandoffError } from '@/lib/produce/crear-handoff-error';
import { crearJobFor } from '@/components/matches/open-in-crear-button';
import type { CreateStepTarget } from '@/components/studio/create-flow-steps';

/** A back control in a Crear flow: "Atrás" for the step before, the destination's name when it leaves the flow. */
export type ProduceBack = CreateStepTarget & { readonly label: string };

export type ProduceBackOptions = { jobId: string; format: ProduceFormat; seriesId: string | null };

/**
 * The player step of a job that already has one: a parsed job cannot re-pick, so this opens the picker on an
 * unpicked job of the same library demo (as "Cambiar jugador" does), or the demo list when the demo is not linked.
 * `from` lets the picker preselect this job's player and reopen this job when the pick does not change.
 */
function usePlayerStep({ jobId, format }: Omit<ProduceBackOptions, 'seriesId'>): { busy: boolean; open: () => void } {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const open = useCallback(() => {
    setBusy(true);
    getLibraryJobDemo(jobId)
      .then(async (demo) => (demo === null ? flowHref(format) : flowHref(format, { job: await crearJobFor({ demo, format, pickPlayers: true }), from: jobId })))
      .then((href) => router.push(href))
      .catch((err: unknown) => {
        const { title, description } = crearHandoffError(err);
        toast.error(title, { description });
        setBusy(false);
      });
  }, [jobId, format, router]);
  return { busy, open };
}

/** The footer's back control and the stepper's finished steps of a producer page. */
export function useProduceBack(options: ProduceBackOptions): { back: ProduceBack; stepTargets: readonly (CreateStepTarget | null)[] } {
  const playerStep = usePlayerStep(options);
  const { format, seriesId } = options;
  const toDemos: CreateStepTarget = { href: flowHref(format) };
  // A series picked its player for every map at once, so its way back is the series page.
  if (seriesId !== null) return { back: { label: 'Serie', href: seriesHref(seriesId, format) }, stepTargets: [toDemos, null] };
  const toPlayer: CreateStepTarget = { onSelect: playerStep.open, busy: playerStep.busy };
  return { back: { label: 'Atrás', ...toPlayer }, stepTargets: [toDemos, toPlayer] };
}
