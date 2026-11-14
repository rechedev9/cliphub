'use client';

import { useId, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Plus, X } from 'lucide-react';
import { MAX_TAG_LENGTH, MAX_TAGS } from '@/lib/api/libreta';
import { patchLibraryDemo, type LibraryDemo } from '@/lib/api/library';
import { cleanTag, tagSuggestions } from '@/lib/library/partida-groups';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

/** Row edit controls stay laid out but show only while the row is hovered or holds focus; a touch screen always shows them. */
export const ROW_EDIT_REVEAL = 'opacity-0 group-hover/row:opacity-100 group-focus-within/row:opacity-100 pointer-coarse:opacity-100';
const ROW_TAG_REVEAL = 'hidden group-hover/row:inline-flex group-focus-within/row:inline-flex pointer-coarse:inline-flex';

/** The tags of a demo as quiet chips, removable and with "+ Etiqueta" while the row is hovered or focused. */
export function PartidaTags({
  demo,
  allTags,
  onChanged,
  quietTags = [],
}: {
  demo: LibraryDemo;
  allTags: readonly string[];
  onChanged: (demoId: string, tags: string[]) => void;
  /** Tags the row already says elsewhere, such as the own team's name: shown only while editing. */
  quietTags?: readonly string[];
}): ReactNode {
  const listId = useId();
  const [adding, setAdding] = useState(false);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  async function save(tags: string[]): Promise<boolean> {
    setBusy(true);
    setFailed(false);
    try {
      const saved = await patchLibraryDemo(demo.id, { tags });
      onChanged(demo.id, saved.tags);
      return true;
    } catch {
      setFailed(true);
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function commit(): Promise<void> {
    const tag = cleanTag(draft);
    if (tag === '' || demo.tags.includes(tag)) {
      setAdding(false);
      setDraft('');
      return;
    }
    if (demo.tags.length >= MAX_TAGS) {
      setFailed(true);
      return;
    }
    if (await save([...demo.tags, tag])) {
      setAdding(false);
      setDraft('');
    }
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      void commit();
    } else if (event.key === 'Escape') {
      setAdding(false);
      setDraft('');
    }
  }

  const suggestions = tagSuggestions({ all: allTags, typed: draft, current: demo.tags });

  return (
    <span className="flex flex-wrap items-center gap-1.5">
      {demo.tags.map((tag) => (
        <Badge
          key={tag}
          variant="secondary"
          shape="square"
          className={cn('gap-0.5 border-transparent pr-0.5 font-normal text-fg-2', quietTags.includes(tag) && ROW_TAG_REVEAL)}
        >
          <span className="max-w-44 truncate" title={tag}>{tag}</span>
          <button
            type="button"
            disabled={busy}
            aria-label={`Quitar la etiqueta ${tag}`}
            onClick={() => void save(demo.tags.filter((other) => other !== tag))}
            className={cn('inline-flex size-5 items-center justify-center rounded-sm text-fg-3 hover:text-fg-1 disabled:opacity-50', FOCUS_RING, ROW_EDIT_REVEAL)}
          >
            <X aria-hidden className="size-3" />
          </button>
        </Badge>
      ))}
      {adding ? (
        <>
          <Input
            autoFocus
            value={draft}
            maxLength={MAX_TAG_LENGTH}
            list={listId}
            disabled={busy}
            aria-label="Nueva etiqueta"
            placeholder="Etiqueta"
            className="h-8 w-36 text-body-sm"
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={onKeyDown}
            onBlur={() => {
              if (draft.trim() === '') setAdding(false);
            }}
          />
          <datalist id={listId}>
            {suggestions.map((tag) => (
              <option key={tag} value={tag} />
            ))}
          </datalist>
        </>
      ) : (
        <Button type="button" variant="ghost" size="icon-xs" title="Añadir etiqueta" aria-label="Añadir etiqueta" className={ROW_EDIT_REVEAL} onClick={() => setAdding(true)}>
          <Plus aria-hidden />
        </Button>
      )}
      {failed ? (
        <span role="alert" className="text-meta text-destructive">
          {demo.tags.length >= MAX_TAGS ? 'Tiene el máximo de etiquetas.' : 'No se pudo guardar la etiqueta.'}
        </span>
      ) : null}
    </span>
  );
}
