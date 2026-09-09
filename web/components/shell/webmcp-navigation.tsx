'use client';

import { useEffect, useLayoutEffect, useRef } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { NAV_SECTIONS } from '@/lib/nav';
import { previewModelContext, registerWebTools, waitForWebState } from '@/lib/webmcp';

export function WebMCPNavigation(): null {
  const pathname = usePathname();
  const router = useRouter();
  const current = useRef({ pathname, router });
  useLayoutEffect(() => { current.current = { pathname, router }; }, [pathname, router]);
  useEffect(() => registerWebTools(previewModelContext(), [
    {
      name: 'cliphub_get_navigation',
      description: 'Read the current ClipHub section and available application sections.',
      inputSchema: { type: 'object', properties: {}, additionalProperties: false },
      annotations: { readOnlyHint: true },
      execute: () => JSON.stringify({ pathname: current.current.pathname, sections: NAV_SECTIONS }),
    },
    {
      name: 'cliphub_navigate',
      description: 'Open a ClipHub section through the app router. Does not create or render media.',
      inputSchema: { type: 'object', properties: { href: { type: 'string', enum: NAV_SECTIONS.map(s => s.href) } }, required: ['href'], additionalProperties: false },
      execute: async ({ href }) => {
        const section = NAV_SECTIONS.find(s => s.href === href);
        if (!section) throw new Error('Unknown ClipHub section');
        current.current.router.push(section.href);
        await waitForWebState(() => current.current.pathname === section.href);
        return JSON.stringify({ pathname: current.current.pathname });
      },
    },
  ]), []);
  return null;
}
