import type { Metadata } from 'next';

// The root template appends " · ClipHub"; a page title that already carried the
// product name read "Táctica — ClipHub Studio · ClipHub". The workspace refines
// this to "<Map> · Táctica" once it knows the demo.
export const metadata: Metadata = { title: 'Táctica' };
export { default } from '@/components/shell/route-layout';
