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

/** The tags of a demo as chips that can be removed, and "+ Etiqueta" to add one with the library's tags as suggestions. */
export function PartidaTags({
  demo,
  allTags,
  onChanged,
}: {
  demo: LibraryDemo;
  allTags: readonly string[];
  onChanged: (demoId: string, tags: string[]) => void;
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
        <Badge key={tag} variant="outline" shape="square" className="gap-0.5 pr-0.5">
          <span className="max-w-32 truncate" title={tag}>{tag}</span>
          <button
            type="button"
            disabled={busy}
            aria-label={`Quitar la etiqueta ${tag}`}
            onClick={() => void save(demo.tags.filter((other) => other !== tag))}
            className={cn('inline-flex size-5 items-center justify-center rounded-sm text-fg-3 hover:text-fg-1 disabled:opacity-50', FOCUS_RING)}
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
        <Button type="button" variant="ghost" size="xs" onClick={() => setAdding(true)}>
          <Plus aria-hidden />
          Etiqueta
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
