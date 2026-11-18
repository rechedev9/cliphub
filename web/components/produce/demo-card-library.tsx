'use client';

import { useState, type ReactNode } from 'react';
import { Pencil } from 'lucide-react';
import type { LibraryDemo } from '@/lib/api/library';
import { demoHeading } from '@/lib/library/demo-title';
import { SOURCE_LABEL } from '@/lib/library/partidas';
import { cn } from '@/lib/utils';
import { DemoTitleEditor } from '@/components/matches/demo-title-editor';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

/** Tags a card shows before it folds the rest into "+N". */
const CARD_TAGS = 3;

type DemoCardHeadingProps = {
  /** The library demo behind the card; without one there is no title to edit. */
  demo: LibraryDemo | undefined;
  teams: string;
  onSaved: (saved: LibraryDemo) => void;
};

/** The first lines of a Crear card: the title, else "A vs B", with a pencil that edits the title. */
export function DemoCardHeading({ demo, teams, onSaved }: DemoCardHeadingProps): ReactNode {
  const [editing, setEditing] = useState(false);
  if (editing && demo !== undefined) return <DemoTitleEditor demo={demo} onSaved={onSaved} onClose={() => setEditing(false)} />;
  const { heading, secondary } = demoHeading({ title: demo?.title, teams, fallback: '' });
  if (heading === '' && demo === undefined) return null;
  const titled = demo?.title !== undefined && demo.title !== '';
  const action = titled ? 'Cambiar el título' : 'Poner título';
  return (
    <div className="flex items-start gap-1">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        {heading === '' ? null : <p className={cn('wrap-anywhere', titled ? 'text-body font-semibold text-fg-1' : 'text-body-sm text-fg-2')}>{heading}</p>}
        {secondary === '' ? null : <p className="text-body-sm wrap-anywhere text-fg-2">{secondary}</p>}
      </div>
      {demo === undefined ? null : (
        <Button type="button" variant="ghost" size="icon-xs" title={action} aria-label={action} className="-mr-1 shrink-0 text-fg-3" onClick={() => setEditing(true)}>
          <Pencil aria-hidden />
        </Button>
      )}
    </div>
  );
}

/** Where the demo came from, named as in Partidas, and its first tags; the rest fold into "+N". */
export function DemoLabels({ demo }: { demo: LibraryDemo }): ReactNode {
  const shown = demo.tags.slice(0, CARD_TAGS);
  const folded = demo.tags.slice(CARD_TAGS);
  const tagClass = 'border-transparent font-normal text-fg-2';
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Badge variant="outline" shape="square" className={demo.source === 'unknown' ? 'text-fg-3' : undefined}>
        {SOURCE_LABEL[demo.source]}
      </Badge>
      {shown.map((tag) => (
        <Badge key={tag} variant="secondary" shape="square" className={tagClass}>
          <span className="max-w-32 truncate" title={tag}>
            {tag}
          </span>
        </Badge>
      ))}
      {folded.length > 0 ? (
        <Badge variant="secondary" shape="square" className={tagClass} title={folded.join(', ')}>
          +{folded.length}
          <span className="sr-only">{` etiquetas más: ${folded.join(', ')}`}</span>
        </Badge>
      ) : null}
    </div>
  );
}
