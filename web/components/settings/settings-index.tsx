'use client';

import { useEffect, type ReactNode } from 'react';
import { FOCUS_RING } from '@/components/ui/button';
import { pinInView } from '@/hooks/pin-in-view';

export type SettingsIndexEntry = { id: string; label: string };

export function SettingsIndex({ entries }: { entries: readonly SettingsIndexEntry[] }): ReactNode {
  useEffect(() => {
    let unpin = (): void => {};
    const reveal = (): void => {
      const id = decodeURIComponent(window.location.hash.slice(1));
      if (!entries.some((entry) => entry.id === id)) return;
      const section = document.getElementById(id);
      if (section === null) return;
      section.scrollIntoView({ block: 'start' });
      unpin();
      unpin = pinInView(section);
    };
    reveal();
    window.addEventListener('hashchange', reveal);
    return () => {
      window.removeEventListener('hashchange', reveal);
      unpin();
    };
  }, [entries]);

  return (
    <nav aria-label="Secciones de Ajustes">
      <ul className="flex flex-wrap gap-x-1 gap-y-1">
        {entries.map((entry) => (
          <li key={entry.id}>
            <a
              href={`#${entry.id}`}
              className={`inline-flex min-h-9 items-center rounded-md px-2.5 text-body-sm text-fg-2 transition-[color,background-color] hover:bg-surface-2 hover:text-fg-1 ${FOCUS_RING}`}
            >
              {entry.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
