import type { Metadata } from 'next';

// Re-declare the suffix: a string title here would end the template for the producer and publish pages.
export const metadata: Metadata = { title: { default: 'Partida', template: '%s · ClipHub' } };
export { default } from '@/components/shell/route-layout';
