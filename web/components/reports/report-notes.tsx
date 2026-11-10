'use client';

import { useMemo, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { Pencil, Play, X } from 'lucide-react';
import { patchLibraryReport } from '@/lib/api/library';
import { MAX_REPORT_NOTES, type LibraryReport, type ReportRoundList } from '@/lib/api/scout';
import { parseMarkdown } from '@/lib/algorithms/markdown';
import { roundListHref } from '@/lib/reports/round-lists';
import { MarkdownBlocks } from '@/components/markdown/markdown-blocks';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';

/** The report's plan in Markdown, and its named round lists, each one a playlist in the Replay. */
export function ReportNotes({ report, onSaved }: { report: LibraryReport; onSaved: () => void }): ReactNode {
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <NotesSection report={report} onSaved={onSaved} />
      <RoundListsSection report={report} onSaved={onSaved} />
    </div>
  );
}

function NotesSection({ report, onSaved }: { report: LibraryReport; onSaved: () => void }): ReactNode {
  const [editing, setEditing] = useState(report.notes.trim() === '');
  const [notes, setNotes] = useState(report.notes);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const blocks = useMemo(() => parseMarkdown(report.notes), [report.notes]);
  const dirty = notes !== report.notes;

  async function save(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await patchLibraryReport(report.id, { notes });
      if (notes.trim() !== '') setEditing(false);
      onSaved();
    } catch {
      setError('No se pudieron guardar las notas.');
    } finally {
      setBusy(false);
    }
  }

  if (!editing) {
    return (
      <section className="flex flex-col gap-3">
        <article className="flex flex-col gap-4">
          <MarkdownBlocks blocks={blocks} />
        </article>
        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" variant="outline" size="sm" onClick={() => setEditing(true)}>
            <Pencil aria-hidden />
            Editar notas
          </Button>
          <span className="text-meta text-fg-3">Salen al principio del informe exportado.</span>
        </div>
      </section>
    );
  }

  return (
    <form onSubmit={(event) => void save(event)} className="flex flex-col gap-3">
      <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
        El plan contra este rival y lo que hay que vigilar
        <Textarea value={notes} maxLength={MAX_REPORT_NOTES} rows={14} onChange={(event) => setNotes(event.target.value)} />
      </label>
      <p className="text-meta text-fg-3">
        Admite Markdown: # títulos, - listas, **negrita** y tablas. Hasta {MAX_REPORT_NOTES.toLocaleString('es-ES')} caracteres.
      </p>
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" size="sm" disabled={busy || !dirty}>
          Guardar notas
        </Button>
        {report.notes.trim() === '' ? null : (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={busy}
            onClick={() => {
              setNotes(report.notes);
              setEditing(false);
            }}
          >
            Cancelar
          </Button>
        )}
      </div>
    </form>
  );
}

function RoundListsSection({ report, onSaved }: { report: LibraryReport; onSaved: () => void }): ReactNode {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  if (report.round_lists.length === 0) return null;

  async function remove(list: ReportRoundList): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      await patchLibraryReport(report.id, { round_lists: report.round_lists.filter((other) => other !== list) });
      onSaved();
    } catch {
      setError('No se pudo quitar la lista.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section aria-labelledby="report-round-lists" className="flex flex-col gap-3">
      <h2 id="report-round-lists" className="text-title font-semibold text-fg-1">
        Listas de rondas
      </h2>
      <ul className="flex flex-col divide-y divide-border border border-border">
        {report.round_lists.map((list, index) => (
          <li key={`${index}-${list.title}`} className="flex flex-wrap items-center gap-3 px-3 py-2">
            <span className="min-w-0 flex-1 text-body text-fg-1">{list.title}</span>
            <span className="text-meta text-fg-3">{list.rounds.length === 1 ? '1 ronda' : `${list.rounds.length} rondas`}</span>
            <Button asChild size="sm" variant="outline">
              <Link href={roundListHref(list)}>
                <Play aria-hidden />
                Ver en el Replay
              </Link>
            </Button>
            <Button type="button" size="sm" variant="ghost" disabled={busy} aria-label={`Quitar la lista ${list.title}`} onClick={() => void remove(list)}>
              <X aria-hidden />
            </Button>
          </li>
        ))}
      </ul>
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
    </section>
  );
}
