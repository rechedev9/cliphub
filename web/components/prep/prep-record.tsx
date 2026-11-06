'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { ArrowLeft, Printer } from 'lucide-react';
import { getTeamPlaybook, getTeamPrep } from '@/lib/api/library';
import type { TeamPlaybook, TeamPrep } from '@/lib/api/playbook';
import { scopeCountText, scopeDateRange } from '@/lib/analysis/scope';
import { PREP_QUERY, REPORTS_HREF, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { MapThumb } from '@/components/analysis/map-thumb';
import { SkippedNote } from '@/components/playbook/playbook-summary';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { CtPositions, MatchupTables, PlaybookBrief, PrepSection, TempoByBuy } from './prep-sections';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const MAP_RE = /^[a-z0-9_]{1,64}$/;
const DAY_RE = /^\d{4}-\d{2}-\d{2}$/;

/** Light ink on paper whatever the app theme, with bar fills kept. */
const PRINT_PAPER = [
  'print:[print-color-adjust:exact]',
  'print:[--background:#fff] print:[--surface-2:#fff] print:[--surface-3:#f1f1f3]',
  'print:[--fg-1:#16171a] print:[--fg-2:#45474d] print:[--fg-3:#5c5e66] print:[--fg-4:#b4b6bc]',
  'print:[--border:#d6d7db] print:[--border-subtle:#e6e7ea] print:[--primary:#16171a]',
].join(' ');

type Loaded = { prep: TeamPrep; t: TeamPlaybook; ct: TeamPlaybook };

function periodText(from: string, to: string): string {
  if (from !== '' && to !== '') return `Del ${formatShortDate(from)} al ${formatShortDate(to)}`;
  if (from !== '') return `Desde el ${formatShortDate(from)}`;
  if (to !== '') return `Hasta el ${formatShortDate(to)}`;
  return 'Todas las fechas';
}

/** The printable prep of a rival on one map, read from `?team=&map=&from=&to=`. */
export function PrepRecord(): ReactNode {
  const query = useSearchParams();
  const teamId = query.get(PREP_QUERY.team) ?? '';
  const map = query.get(PREP_QUERY.map) ?? '';
  const fromDay = DAY_RE.test(query.get(PREP_QUERY.from) ?? '') ? (query.get(PREP_QUERY.from) ?? '') : '';
  const toDay = DAY_RE.test(query.get(PREP_QUERY.to) ?? '') ? (query.get(PREP_QUERY.to) ?? '') : '';
  const valid = UUID_RE.test(teamId) && MAP_RE.test(map);
  const [data, setData] = useState<Loaded | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const title = data ? `Preparar a ${data.prep.team_name}` : 'Preparar rival';
  useRouteTitle(title);
  useDocumentTitle(`${title} · Informes · ClipHub`);

  useEffect(() => {
    if (!valid) return undefined;
    let active = true;
    setData(null);
    setFailed(false);
    const { from, to } = scopeDateRange({ kind: 'range', from: fromDay, to: toDay }, Date.now());
    const base = { map, from, to };
    Promise.all([getTeamPrep(teamId, base), getTeamPlaybook(teamId, { ...base, side: 'T' }), getTeamPlaybook(teamId, { ...base, side: 'CT' })])
      .then(([prep, t, ct]) => {
        if (active) setData({ prep, t, ct });
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [valid, teamId, map, fromDay, toDay, attempt]);

  const back = (
    <Button asChild variant="ghost" size="sm">
      <Link href={REPORTS_HREF}>
        <ArrowLeft aria-hidden />
        Informes
      </Link>
    </Button>
  );

  if (!valid) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p className="text-body text-fg-2">Falta el rival o el mapa. Vuelve a Informes y pulsa Preparar rival.</p>
        {back}
      </div>
    );
  }
  if (failed) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p role="alert" className="text-body-sm text-destructive">
          No se pudo preparar este rival. Comprueba que ClipHub Studio sigue abierto y que el equipo sigue en la biblioteca.
        </p>
        <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
          Reintentar
        </Button>
      </div>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Preparando el rival" className="flex flex-col gap-5">
        <Skeleton className="h-12 w-80" />
        <Skeleton className="h-56 w-full" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-72 w-full" />
      </div>
    );
  }

  const { prep, t, ct } = data;
  return (
    <article className={`flex flex-col gap-5 print:gap-6 ${PRINT_PAPER}`}>
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-3">
        <div className="flex items-center gap-3">
          <MapThumb map={prep.map} className="h-12 w-20 print:hidden" crop={1.3} />
          <div className="flex flex-col gap-0.5">
          <h1 className="text-title font-semibold text-fg-1">
            <Link href={teamHref(prep.team_id)} className="hover:underline print:no-underline">
              {prep.team_name}
            </Link>{' '}
            en {prettyMapName(prep.map)}
          </h1>
          <p className="text-body-sm text-fg-2 tabular-nums">
            {periodText(fromDay, toDay)} · {scopeCountText(prep.matches, prep.round_count)}
          </p>
          </div>
        </div>
        <div className="flex items-center gap-2 print:hidden">
          {back}
          <Button type="button" size="sm" onClick={() => window.print()}>
            <Printer aria-hidden />
            Imprimir
          </Button>
        </div>
      </header>
      {prep.round_count === 0 ? (
        <p className="text-body text-fg-2">
          No hay rondas de {prep.team_name} en {prettyMapName(prep.map)} en este periodo. Prueba con todas las fechas.
        </p>
      ) : (
        <>
          <PrepSection title="Compras contra compras">
            <MatchupTables prep={prep} />
          </PrepSection>
          <PrepSection title="Ritmo por compra">
            <TempoByBuy prep={prep} />
          </PrepSection>
          <PrepSection title="Posiciones CT por tramo">
            <CtPositions prep={prep} />
          </PrepSection>
          <PrepSection title="Playbook en T">
            <PlaybookBrief playbook={t} />
          </PrepSection>
          <PrepSection title="Playbook en CT">
            <PlaybookBrief playbook={ct} />
          </PrepSection>
        </>
      )}
      <SkippedNote skipped={prep.skipped} />
    </article>
  );
}
