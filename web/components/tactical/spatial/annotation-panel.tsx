'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { MessageSquarePlus, PenLine, Trash2 } from 'lucide-react';
import { createAnnotation, deleteAnnotation, listAnnotations, patchAnnotation } from '@/lib/api/library';
import type { AnnotationInput } from '@/lib/api/library';
import { ANNOTATION_TEXT_MAX } from '@/lib/api/spatial';
import type { Annotation } from '@/lib/api/spatial';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { TelestratorToolbar } from './telestrator';
import type { TelestratorDraft } from './telestrator';

const TEXTAREA_CLASS =
  'min-h-16 w-full rounded-md border border-input bg-transparent px-2.5 py-1.5 text-body-sm text-fg-1 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50';

function byAnchor(a: Annotation, b: Annotation): number {
  if (a.match_id !== b.match_id) return a.match_id < b.match_id ? -1 : 1;
  if (a.round !== b.round) return a.round - b.round;
  if (a.tick !== b.tick) return a.tick - b.tick;
  if (a.created_at !== b.created_at) return a.created_at < b.created_at ? -1 : 1;
  return a.id < b.id ? -1 : 1;
}

export type AnnotationStore = {
  notes: Annotation[] | null;
  error: string | null;
  create: (input: AnnotationInput) => Promise<boolean>;
  update: (id: string, text: string) => Promise<boolean>;
  remove: (id: string) => Promise<void>;
};

/** The notes of a set of matches (one round, or all), kept in anchor order after each write. */
export function useAnnotations(matchIds: readonly string[], round?: number): AnnotationStore {
  const [notes, setNotes] = useState<Annotation[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const key = matchIds.join(',');

  useEffect(() => {
    let active = true;
    setNotes(null);
    setError(null);
    listAnnotations(key === '' ? [] : key.split(','), round)
      .then((list) => {
        if (active) setNotes(list);
      })
      .catch(() => {
        if (active) {
          setNotes([]);
          setError('No se pudieron leer las notas. Comprueba que ClipHub Studio sigue abierto.');
        }
      });
    return () => {
      active = false;
    };
  }, [key, round]);

  const create = useCallback(async (input: AnnotationInput) => {
    try {
      const saved = await createAnnotation(input);
      setNotes((current) => [...(current ?? []), saved].sort(byAnchor));
      setError(null);
      return true;
    } catch {
      setError('No se pudo guardar la nota.');
      return false;
    }
  }, []);

  const update = useCallback(async (id: string, text: string) => {
    try {
      const saved = await patchAnnotation(id, { text });
      setNotes((current) => (current ?? []).map((note) => (note.id === id ? saved : note)));
      setError(null);
      return true;
    } catch {
      setError('No se pudo editar la nota.');
      return false;
    }
  }, []);

  const remove = useCallback(async (id: string) => {
    try {
      await deleteAnnotation(id);
      setNotes((current) => (current ?? []).filter((note) => note.id !== id));
      setError(null);
    } catch {
      setError('No se pudo borrar la nota.');
    }
  }, []);

  return { notes, error, create, update, remove };
}

/** One listed note: where it is anchored, and how to go there. */
export type NoteItem = {
  note: Annotation;
  /** "R4 · 0:23", the anchor as the reader sees it. */
  label: string;
  active: boolean;
  onSeek?: () => void;
  href?: string;
};

/**
 * Notes anchored to a tick: the new-note form with its drawing tools, then the
 * list. A note is saved at the moment the playhead is on; its drawing shows on
 * the radar around that moment.
 */
export function AnnotationPanel({
  store,
  items,
  draft,
  anchorLabel,
  onCreate,
}: {
  store: AnnotationStore;
  items: readonly NoteItem[];
  draft: TelestratorDraft;
  /** Where a new note goes, or null when there is no round to anchor it to. */
  anchorLabel: string | null;
  onCreate: (text: string) => Promise<boolean>;
}): ReactNode {
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const canSave = anchorLabel !== null && (text.trim() !== '' || draft.strokes.length > 0) && !busy;

  const submit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (!canSave) return;
    setBusy(true);
    const ok = await onCreate(text);
    setBusy(false);
    if (ok) {
      setText('');
      draft.clear();
      draft.setDrawing(false);
    }
  };

  return (
    <section aria-label="Notas del replay" className="flex flex-col gap-3">
      <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-2">
        <TelestratorToolbar draft={draft} />
        <textarea
          value={text}
          maxLength={ANNOTATION_TEXT_MAX}
          rows={2}
          placeholder="Nota para el equipo"
          aria-label="Texto de la nota"
          onChange={(event) => setText(event.target.value)}
          className={TEXTAREA_CLASS}
        />
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="font-mono text-meta tabular-nums text-fg-3">{anchorLabel ?? 'Sin ronda para anclar la nota'}</span>
          <Button type="submit" size="xs" disabled={!canSave}>
            <MessageSquarePlus aria-hidden />
            Guardar nota
          </Button>
        </div>
      </form>
      {store.error ? (
        <p role="alert" className="text-body-sm text-destructive">
          {store.error}
        </p>
      ) : null}
      <NoteList store={store} items={items} />
    </section>
  );
}

function NoteList({ store, items }: { store: AnnotationStore; items: readonly NoteItem[] }): ReactNode {
  if (store.notes === null) return <p className="text-body-sm text-fg-3">Leyendo notas…</p>;
  if (items.length === 0) {
    return <p className="text-body-sm text-fg-3">Aún no hay notas aquí. Pausa en un momento, dibuja y guarda.</p>;
  }
  return (
    <ul className="flex flex-col divide-y divide-border-subtle">
      {items.map((item) => (
        <NoteRow key={item.note.id} item={item} store={store} />
      ))}
    </ul>
  );
}

function NoteRow({ item, store }: { item: NoteItem; store: AnnotationStore }): ReactNode {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(item.note.text);
  const { note } = item;
  const strokes = note.strokes.length;

  const save = async (): Promise<void> => {
    if (await store.update(note.id, text)) setEditing(false);
  };

  return (
    <li className={cn('flex flex-col gap-1 py-2', item.active && 'border-l-2 border-l-primary pl-2')}>
      <div className="flex items-center justify-between gap-2">
        {item.onSeek ? (
          <button
            type="button"
            onClick={item.onSeek}
            className="font-mono text-meta tabular-nums text-fg-2 underline-offset-2 outline-none hover:text-fg-1 hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            {item.label}
          </button>
        ) : (
          <span className="font-mono text-meta tabular-nums text-fg-2">{item.label}</span>
        )}
        <span className="flex items-center gap-1">
          {item.href ? (
            <Link href={item.href} className="text-meta text-fg-3 underline-offset-2 hover:text-fg-1 hover:underline">
              Abrir ronda
            </Link>
          ) : null}
          <Button type="button" size="icon-xs" variant="ghost" aria-label="Editar nota" onClick={() => setEditing(!editing)}>
            <PenLine aria-hidden />
          </Button>
          <Button type="button" size="icon-xs" variant="ghost" aria-label="Borrar nota" onClick={() => void store.remove(note.id)}>
            <Trash2 aria-hidden />
          </Button>
        </span>
      </div>
      {editing ? (
        <div className="flex flex-col gap-1.5">
          <textarea
            value={text}
            maxLength={ANNOTATION_TEXT_MAX}
            rows={2}
            aria-label="Editar el texto de la nota"
            onChange={(event) => setText(event.target.value)}
            className={TEXTAREA_CLASS}
          />
          <div>
            <Button type="button" size="xs" disabled={text.trim() === '' && strokes === 0} onClick={() => void save()}>
              Guardar
            </Button>
          </div>
        </div>
      ) : (
        <p className="text-body-sm whitespace-pre-wrap break-words text-fg-1">
          {note.text === '' ? <span className="text-fg-3">Solo dibujo</span> : note.text}
          {strokes > 0 && note.text !== '' ? <span className="pl-1.5 text-meta text-fg-3">· dibujo de {strokes} {strokes === 1 ? 'trazo' : 'trazos'}</span> : null}
        </p>
      )}
    </li>
  );
}
