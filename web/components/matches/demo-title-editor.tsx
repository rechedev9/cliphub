'use client';

import { useState, type KeyboardEvent, type ReactNode } from 'react';
import { patchLibraryDemo, type LibraryDemo } from '@/lib/api/library';
import { MAX_DEMO_TITLE_LENGTH, clipDemoTitle } from '@/lib/library/demo-title';
import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';

type DemoTitleEditorProps = {
  demo: LibraryDemo;
  onSaved: (saved: LibraryDemo) => void;
  onClose: () => void;
  className?: string;
};

/** The title of a library demo as one text field: Intro saves, Esc leaves it as it was, and an empty title clears it. */
export function DemoTitleEditor({ demo, onSaved, onClose, className }: DemoTitleEditorProps): ReactNode {
  const [draft, setDraft] = useState(demo.title ?? '');
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const full = [...draft].length >= MAX_DEMO_TITLE_LENGTH;

  async function save(): Promise<void> {
    if (draft.trim() === (demo.title ?? '')) {
      onClose();
      return;
    }
    setBusy(true);
    setFailed(false);
    try {
      onSaved(await patchLibraryDemo(demo.id, { title: draft }));
      onClose();
    } catch {
      setFailed(true);
      setBusy(false);
    }
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      if (!busy) void save();
    } else if (event.key === 'Escape') {
      event.preventDefault();
      onClose();
    }
  }

  return (
    <span className={cn('flex min-w-0 flex-col gap-1', className)}>
      <Input
        autoFocus
        value={draft}
        readOnly={busy}
        aria-label="Título de la demo"
        placeholder="Título de la demo"
        className="h-8 text-body-sm"
        onChange={(event) => setDraft(clipDemoTitle(event.target.value))}
        onKeyDown={onKeyDown}
      />
      <span aria-live="polite" className={cn('text-meta', full ? 'text-warning' : 'text-fg-3')}>
        {full ? `Has llegado al máximo de ${MAX_DEMO_TITLE_LENGTH} caracteres.` : 'Intro para guardar, Esc para cancelar.'}
      </span>
      {failed ? (
        <span role="alert" className="text-meta text-destructive">
          No se pudo guardar el título.
        </span>
      ) : null}
    </span>
  );
}
