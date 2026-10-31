import type { Metadata } from 'next';

// A plain string here ends the root template for the publish page nested under it.
export const metadata: Metadata = { title: { default: 'Mis vídeos', template: '%s · ClipHub' } };
export { default } from '@/components/shell/route-layout';
