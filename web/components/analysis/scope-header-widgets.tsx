'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ChevronDown } from 'lucide-react';
import { BASELINE_SCOPES, type Baseline } from '@/lib/api/baseline';
import type { SampleNotes } from '@/lib/api/team-insights';
import { resultRecord, type ResultDot } from '@/lib/analysis/context-bar';
import { SCOPE_LABEL, proReferenceShort, proSourceText } from '@/lib/analysis/reference';
import { SCOPE_LIMIT_MAX, scopeCountText, type AnalysisScope } from '@/lib/analysis/scope';
import type { ScopeCounts, ScopeMatch } from '@/components/analysis/scope-bar';
import { CAPS_LABEL } from '@/components/analysis/scope-field-style';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Segmented } from '@/components/ui/segmented';
import { cn } from '@/lib/utils';

const MAX_DOTS = 10;
const DOT: Record<ResultDot, string> = { win: 'bg-outcome-win', loss: 'bg-outcome-loss', draw: 'bg-fg-3' };
const DOT_WORD: Record<ResultDot, string> = { win: 'Victoria', loss: 'Derrota', draw: 'Empate' };

/** A small bordered box of the header's right side, titled with a functional caps label. */
export function HeaderWidget({ title, children, className }: { title: string; children: ReactNode; className?: string }): ReactNode {
  return (
    <section aria-label={title} className={cn('flex min-w-0 flex-col items-center gap-1 border border-border-strong bg-surface-2 px-2.5 py-1.5', className)}>
      <h2 className={cn(CAPS_LABEL, 'whitespace-nowrap')}>{title}</h2>
      {children}
    </section>
  );
}

/** "Comparando con": the library mean or the pros'. */
export function ReferenceWidget({
  scope,
  onChange,
  proReference,
}: {
  scope: AnalysisScope;
  onChange: (patch: Partial<AnalysisScope>) => void;
  proReference: Baseline | null | undefined;
}): ReactNode {
  return (
    <HeaderWidget title="Comparando con">
      <Segmented
        label="Comparando con"
        value={scope.reference}
        options={BASELINE_SCOPES.map((value) => ({ value, label: SCOPE_LABEL[value] }))}
        onChange={(reference) => onChange({ reference })}
      />
      {scope.reference === 'pro' && proReference ? (
        <span className="text-meta whitespace-nowrap text-fg-3" title={proSourceText(proReference)}>
          {proReferenceShort(proReference)}
        </span>
      ) : null}
    </HeaderWidget>
  );
}

function SampleNoteList({ notes }: { notes: SampleNotes }): ReactNode {
  const items = [
    { count: notes.unrated, text: `${notes.unrated} sin ranking`, title: 'Partidas fuera porque el rival no tiene ranking HLTV' },
    { count: notes.capped, text: `${notes.capped} fuera por el tope de ${SCOPE_LIMIT_MAX}`, title: `Se leen como mucho las ${SCOPE_LIMIT_MAX} partidas más recientes` },
    { count: notes.unread, text: `${notes.unread} sin leer por el tope de 200`, title: 'Se abren como mucho 200 análisis por vista: acota el mapa, las fechas o la alineación para leer el resto' },
    {
      count: notes.awpEstimated,
      text: `AWP estimado en ${notes.awpEstimated} ${notes.awpEstimated === 1 ? 'partida' : 'partidas'}`,
      title: 'Su análisis es anterior al registro de AWP: se deduce de las bajas y los daños con AWP. Vuelve a analizarlas para el dato exacto.',
    },
  ].filter((item) => item.count > 0);
  return items.map((item) => (
    <p key={item.text} className="text-meta whitespace-nowrap text-fg-3 tabular-nums" title={item.title}>
      {item.text}
    </p>
  ));
}

function ResultDots({ results }: { results: readonly ResultDot[] | undefined }): ReactNode {
  const dots = (results ?? []).slice(-MAX_DOTS);
  if (dots.length === 0) return null;
  return (
    <p className="flex items-center gap-1" title={`Últimos resultados: ${resultRecord(dots)}`}>
      <span className="sr-only">Últimos resultados: {resultRecord(dots)}</span>
      {dots.map((dot, index) => (
        <span key={index} aria-hidden title={DOT_WORD[dot]} className={cn('size-2 rounded-full', DOT[dot])} />
      ))}
    </p>
  );
}

function MatchesPopover({ matches, results }: { matches: readonly ScopeMatch[]; results: readonly ResultDot[] | undefined }): ReactNode {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="xs" className="h-5 gap-1 px-1 normal-case tracking-normal">
          Mostrar partidas
          <ChevronDown aria-hidden />
          <ResultDots results={results} />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="max-h-80 w-80 overflow-y-auto p-0">
        <ul aria-label="Partidas en el filtro" className="divide-y divide-border-subtle">
          {matches.map((match) => (
            <li key={match.id}>
              <Link href={match.href} className="flex items-center gap-2 px-2 py-1.5 text-body-sm text-fg-1 hover:bg-surface-5">
                <span aria-hidden className={cn('size-2 shrink-0 rounded-full', match.outcome === null ? 'bg-fg-4' : DOT[match.outcome])} />
                <span className="sr-only">{match.outcome === null ? 'Sin resultado' : DOT_WORD[match.outcome]}</span>
                <span className="min-w-0 flex-1 truncate">{match.label}</span>
                <span className="text-meta whitespace-nowrap text-fg-3 tabular-nums">{match.date}</span>
              </Link>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  );
}

/** "N MAPAS / N RONDAS", the sample the figures stand on, with the matches behind it and the latest results. */
export function CounterWidget({
  counts,
  results,
  matches,
}: {
  counts: ScopeCounts | null;
  results: readonly ResultDot[] | undefined;
  matches: readonly ScopeMatch[] | undefined;
}): ReactNode {
  return (
    <section aria-live="polite" aria-label="Muestra" className="flex min-w-0 flex-col items-center gap-1 border border-border-strong bg-surface-2 px-3 py-1.5">
      {counts === null ? (
        <p className={CAPS_LABEL}>Contando</p>
      ) : (
        <>
          <p className="sr-only">{scopeCountText(counts.maps, counts.rounds)}</p>
          <div aria-hidden className="flex items-start gap-4">
            <div className="flex flex-col items-center leading-none">
              <span className="text-title font-bold text-fg-1 tabular-nums">{counts.maps}</span>
              <span className={CAPS_LABEL}>{counts.maps === 1 ? 'Mapa' : 'Mapas'}</span>
            </div>
            <div className="flex flex-col items-center leading-none">
              <span className="text-title font-bold text-fg-1 tabular-nums">{counts.rounds}</span>
              <span className={CAPS_LABEL}>{counts.rounds === 1 ? 'Ronda' : 'Rondas'}</span>
            </div>
          </div>
          {counts.notes === undefined ? null : <SampleNoteList notes={counts.notes} />}
        </>
      )}
      {matches !== undefined && matches.length > 0 ? <MatchesPopover matches={matches} results={results} /> : <ResultDots results={results} />}
    </section>
  );
}
