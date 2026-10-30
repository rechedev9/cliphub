import type { Metadata } from 'next';

// Re-declare the suffix: a string title here would end the template for the publish page.
export const metadata: Metadata = { title: { default: 'Vídeo', template: '%s · ClipHub' } };
export { default } from '@/components/shell/route-layout';
