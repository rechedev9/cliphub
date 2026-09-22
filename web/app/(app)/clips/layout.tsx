import type { Metadata } from 'next';

// A plain string here ends the root template for nested routes, so the producer
// and publish pages read "Partida" with no product suffix. Re-declare it.
export const metadata: Metadata = { title: { default: 'Clips y vídeos', template: '%s · ClipHub' } };
export { default } from '@/components/shell/route-layout';
