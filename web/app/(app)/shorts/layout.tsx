import type { Metadata } from 'next';

// A plain string here would end the root template for the producer page nested under it.
export const metadata: Metadata = { title: { default: 'Shorts', template: '%s · ClipHub' } };
export { default } from '@/components/shell/route-layout';
