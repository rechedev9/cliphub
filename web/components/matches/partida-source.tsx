'use client';

import type { ReactNode } from 'react';
import { LIBRARY_SOURCES, type LibraryDemo, type LibrarySource } from '@/lib/api/library';
import { SOURCE_LABEL, withoutDemo } from '@/lib/library/partidas';
import { cn } from '@/lib/utils';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const EDITABLE_SOURCES = LIBRARY_SOURCES.filter((source) => !withoutDemo(source));

/** The source of a demo as a chip that turns into a select while the row is hovered or focused; a shared or pro match keeps its own. */
export function PartidaSource({ demo, onChange }: { demo: LibraryDemo; onChange: (demoId: string, source: LibrarySource) => void }): ReactNode {
  if (withoutDemo(demo.source)) return <span className="text-meta text-fg-3">{SOURCE_LABEL[demo.source]}</span>;
  return (
    <Select value={demo.source} onValueChange={(source) => onChange(demo.id, EDITABLE_SOURCES.find((entry) => entry === source) ?? demo.source)}>
      <SelectTrigger
        aria-label="Fuente de la partida"
        className={cn(
          'h-6 w-auto gap-1 border-transparent bg-surface-3 px-2 text-meta font-semibold tracking-wide shadow-none',
          'group-hover/row:border-border-strong group-focus-within/row:border-border-strong pointer-coarse:border-border-strong',
          '[&>svg]:opacity-0 group-hover/row:[&>svg]:opacity-100 group-focus-within/row:[&>svg]:opacity-100 pointer-coarse:[&>svg]:opacity-100',
        )}
      >
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
