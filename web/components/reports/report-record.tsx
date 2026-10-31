'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowLeft, FileDown, FileText, Pencil, PlugZap, Printer, Trash2 } from 'lucide-react';
import {
  deleteLibraryReport,
  getLibraryReport,
  getLibraryReportMetrics,
  getLibraryReportScout,
  isLibraryNotFound,
  listLibraryMatches,
  patchLibraryReport,
  type LibraryMatch,
} from '@/lib/api/library';
import type { MetricsReport } from '@/lib/api/metrics';
import type { LibraryReport, ReportSide, ScoutReport } from '@/lib/api/scout';
import { REPORTS_HREF, playerHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { buildReportHtml, reportFileStem } from '@/lib/reports/export-html';
import { saveReportPdf } from '@/lib/reports/pdf';
import { REPORT_SIDE_LABEL, matchIndex } from '@/lib/reports/scout-sections';
import { groupEvidenceByMatch, type VideoSource } from '@/lib/library/analysis-video';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { MAX_REPORT_MATCHES, ReportSideSelect } from './new-report-dialog';
import { EMPTY_PICKER_FILTERS, ReportMatchPicker, type PickerFilters } from './report-match-picker';
import { MetricsView } from '@/components/metrics/metrics-view';
import { ScoutView } from './scout-view';
import { ReportSpatial } from './report-spatial';

/** Metrics are `null` when they could not be computed: the report still opens without them. */
type Loaded = { report: LibraryReport; scout: ScoutReport; metrics: MetricsReport | null };
type LoadState = { kind: 'loading' } | { kind: 'ready'; data: Loaded } | { kind: 'missing' } | { kind: 'offline' };

/** A saved scouting report: its sections, its notes and its exports. */
export function ReportRecord({ reportId }: { reportId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    Promise.all([getLibraryReport(reportId), getLibraryReportScout(reportId), getLibraryReportMetrics(reportId).catch(() => null)])
      .then(([report, scout, metrics]) => {
        if (active) setState({ kind: 'ready', data: { report, scout, metrics } });
      })
      .catch((error: unknown) => {
        if (!active) return;
        if (isLibraryNotFound(error)) setState({ kind: 'missing' });
        else setState({ kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [reportId, attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando informe" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={FileText}
        title="Informe no encontrado"
        description="Puede que se haya eliminado o que el enlace sea antiguo."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={REPORTS_HREF}>Ver informes</Link>
          </Button>
        }
      />
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudo leer el informe. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <ReportRecordView data={state.data} onChanged={reload} />;
}

function exportInput(report: LibraryReport, scout: ScoutReport, metrics: MetricsReport | null) {
  return {
    title: report.title,
    teamName: report.team_name,
    side: report.side,
    notes: report.notes,
    scout,
    metrics,
    origin: window.location.origin,
    generatedAt: new Date(),
  };
}

function downloadHtml(html: string, stem: string): void {
  const url = URL.createObjectURL(new Blob([html], { type: 'text/html;charset=utf-8' }));
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `${stem}.html`;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** Outside Studio there is no PDF bridge: print the standalone report from a hidden frame. */
function printHtml(html: string): void {
  const frame = document.createElement('iframe');
  frame.setAttribute('aria-hidden', 'true');
  frame.className = 'fixed size-0 border-0';
  frame.srcdoc = html;
  frame.onload = () => {
    frame.contentWindow?.print();
    setTimeout(() => frame.remove(), 1000);
  };
  document.body.append(frame);
}

function ReportRecordView({ data, onChanged }: { data: Loaded; onChanged: () => void }): ReactNode {
  const router = useRouter();
  const { report, scout, metrics } = data;
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  /** The example rounds of the report row the video dialog opened on. */
  const [video, setVideo] = useState<VideoSource[] | null>(null);
  useRouteTitle(report.title);
  useDocumentTitle(`${report.title} · Informe · ClipHub`);

  const maps = [...new Set(scout.matches.map((match) => prettyMapName(match.map)))];

  function exportHtml(): void {
    downloadHtml(buildReportHtml(exportInput(report, scout, metrics)), reportFileStem(report.title));
    setMessage('Informe exportado como HTML.');
  }

  async function exportPdf(): Promise<void> {
    setExporting(true);
    setMessage(null);
    try {
      const html = buildReportHtml(exportInput(report, scout, metrics));
      const result = await saveReportPdf(html, `${reportFileStem(report.title)}.pdf`);
      if (result.kind === 'unavailable') printHtml(html);
      if (result.kind === 'saved') setMessage('PDF guardado.');
      if (result.kind === 'failed') setMessage(result.message);
    } catch {
      setMessage('No se pudo guardar el PDF.');
    } finally {
      setExporting(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Informe"
        icon={FileText}
        title={report.title}
        subtitle={
          <>
            Rival:{' '}
            <Link href={teamHref(report.team_id)} className="text-fg-1 hover:text-primary hover:underline">
              {report.team_name}
            </Link>
          </>
        }
        fields={[
          { label: 'Partidas', value: scout.matches.length },
          { label: 'Mapas', value: maps.length === 0 ? 'Sin mapas' : maps.join(', ') },
          { label: 'Lado', value: REPORT_SIDE_LABEL[report.side] },
          { label: 'Rondas leídas', value: scout.tendencies.round_count },
          { label: 'Actualizado', value: formatShortDate(report.updated_at) },
        ]}
        actions={
          <>
            <Button type="button" size="sm" onClick={exportHtml}>
              <FileDown aria-hidden />
              Exportar HTML
            </Button>
            <Button type="button" variant="outline" size="sm" disabled={exporting} onClick={() => void exportPdf()}>
              <Printer aria-hidden />
              Guardar PDF
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setEditing(true)}>
              <Pencil aria-hidden />
              Editar
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => setDeleting(true)}>
              <Trash2 aria-hidden />
              Eliminar
            </Button>
            <Button asChild variant="ghost" size="sm">
              <Link href={REPORTS_HREF}>
                <ArrowLeft aria-hidden />
                Todos los informes
              </Link>
            </Button>
          </>
        }
      />
      {message ? (
        <p role="status" className="text-body-sm text-fg-2">
          {message}
        </p>
      ) : null}
      <ScoutView
        scout={scout}
        onRowVideo={(evidence) => setVideo(groupEvidenceByMatch(evidence))}
        leading={[
          { id: 'notas', label: 'Notas', content: <ReportNotes report={report} onSaved={onChanged} /> },
          { id: 'radar', label: 'Radar 2D', content: <ReportSpatial report={report} /> },
          {
            id: 'metricas',
            label: 'Métricas',
            content: metrics ? (
              <MetricsView report={metrics} options={{ playerHref }} matches={matchIndex(scout)} />
            ) : (
              <p className="text-body-sm text-fg-2">No se pudieron calcular las métricas de este informe. Vuelve a abrirlo para reintentarlo.</p>
            ),
          },
        ]}
      />
      {video ? <AnalysisVideoDialog title="Crear vídeo de estas rondas" sources={video} teamId={report.team_id} onClose={() => setVideo(null)} /> : null}
      <EditReportDialog open={editing} onOpenChange={setEditing} report={report} onSaved={onChanged} />
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Eliminar {report.title}</DialogTitle>
            <DialogDescription>Se borra el informe. Las partidas y el equipo siguen en la biblioteca.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleting(false)}>
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => {
                void deleteLibraryReport(report.id)
                  .then(() => router.replace(REPORTS_HREF))
                  .catch(() => setMessage('No se pudo eliminar el informe.'))
                  .finally(() => setDeleting(false));
              }}
            >
              <Trash2 aria-hidden />
              Eliminar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function ReportNotes({ report, onSaved }: { report: LibraryReport; onSaved: () => void }): ReactNode {
  const [notes, setNotes] = useState(report.notes);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dirty = notes !== report.notes;

  async function save(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await patchLibraryReport(report.id, { notes });
      onSaved();
    } catch {
      setError('No se pudieron guardar las notas.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={(event) => void save(event)} className="flex flex-col gap-3">
      <p className="measure-read text-body-sm text-fg-2">
        La lectura del coach: el plan contra este rival y lo que hay que vigilar. Las notas salen al principio del informe exportado.
      </p>
      <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
        Notas
        <textarea
          value={notes}
          maxLength={4000}
          rows={8}
          onChange={(event) => setNotes(event.target.value)}
          className="min-h-40 rounded-md border border-input bg-transparent px-3 py-2 text-body-sm text-fg-1 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
        />
      </label>
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      <div>
        <Button type="submit" size="sm" disabled={busy || !dirty}>
          Guardar notas
        </Button>
      </div>
    </form>
  );
}

function EditReportDialog({
  open,
  onOpenChange,
  report,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  report: LibraryReport;
  onSaved: () => void;
}): ReactNode {
  const [title, setTitle] = useState(report.title);
  const [side, setSide] = useState<ReportSide>(report.side);
  const [matches, setMatches] = useState<LibraryMatch[] | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set(report.match_ids));
  const [filters, setFilters] = useState<PickerFilters>(EMPTY_PICKER_FILTERS);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTitle(report.title);
    setSide(report.side);
    setSelected(new Set(report.match_ids));
    setFilters(EMPTY_PICKER_FILTERS);
    setError(null);
    setMatches(null);
    listLibraryMatches({ team_id: report.team_id })
      .then(setMatches)
      .catch(() => setMatches([]));
  }, [open, report]);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (selected.size === 0 || selected.size > MAX_REPORT_MATCHES) {
      setError(`Elige entre 1 y ${MAX_REPORT_MATCHES} partidas.`);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const known = new Set((matches ?? []).map((match) => match.id));
      // A saved match that left the library is dropped here rather than refused by the server.
      const ids = [...selected].filter((id) => known.has(id));
      await patchLibraryReport(report.id, { title: title.trim(), side, match_ids: ids });
      onOpenChange(false);
      onSaved();
    } catch {
      setError('No se pudo guardar el informe.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-2xl" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Editar informe</DialogTitle>
            <DialogDescription>Cambia el título, el lado o las partidas de {report.team_name} que lee el informe.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Título
              <Input value={title} maxLength={120} onChange={(event) => setTitle(event.target.value)} />
            </label>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Lado
              <ReportSideSelect value={side} onChange={setSide} />
            </label>
          </div>
          {matches === null ? (
            <p role="status" className="text-body-sm text-fg-2">Cargando partidas</p>
          ) : (
            <ReportMatchPicker
              teamId={report.team_id}
              matches={matches}
              selected={selected}
              onSelectedChange={setSelected}
              filters={filters}
              onFiltersChange={setFilters}
            />
          )}
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || title.trim() === '' || selected.size === 0}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
