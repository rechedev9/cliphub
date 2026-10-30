import type { Metadata } from 'next';

// The root template appends " · ClipHub". A partida refines this to
// "<Map> · Partida" once it knows the demo.
export const metadata: Metadata = { title: 'Partidas' };
export { default } from '@/components/shell/route-layout';
