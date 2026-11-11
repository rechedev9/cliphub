import type { ReactNode } from 'react';
import { PartidasSkeleton } from '@/components/matches/match-library';

/** Replaces the generic app skeleton so opening Partidas shows the table it is about to load. */
export default function PartidasLoading(): ReactNode {
  return <PartidasSkeleton />;
}
