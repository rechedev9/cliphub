'use client';

import type { ReactNode } from 'react';
import { LIBRARY_SOURCES, type LibraryDemo, type LibrarySource } from '@/lib/api/library';
import { SOURCE_LABEL, withoutDemo } from '@/lib/library/partidas';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const EDITABLE_SOURCES = LIBRARY_SOURCES.filter((source) => !withoutDemo(source));

/** The source of a demo, changeable from the row; a shared or pro match keeps its own. */
export function PartidaSource({ demo, onChange }: { demo: LibraryDemo; onChange: (demoId: string, source: LibrarySource) => void }): ReactNode {
  if (withoutDemo(demo.source)) return <span className="text-meta text-fg-3">{SOURCE_LABEL[demo.source]}</span>;
  return (
    <Select value={demo.source} onValueChange={(source) => onChange(demo.id, EDITABLE_SOURCES.find((entry) => entry === source) ?? demo.source)}>
      <SelectTrigger aria-label="Fuente de la partida" className="h-8 w-36 text-meta">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {EDITABLE_SOURCES.map((source) => (
          <SelectItem key={source} value={source}>
            {SOURCE_LABEL[source]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
