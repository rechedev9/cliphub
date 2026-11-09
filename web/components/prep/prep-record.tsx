'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { ArrowLeft, ClipboardList, Printer } from 'lucide-react';
import { scopeCountText, scopeDateRange } from '@/lib/analysis/scope';
import { PREP_QUERY, TEAMS_HREF, teamPrepareHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { MapThumb } from '@/components/analysis/map-thumb';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { PrepBody } from './prep-body';

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

function periodText(from: string, to: string): string {
  if (from !== '' && to !== '') return `Del ${formatShortDate(from)} al ${formatShortDate(to)}`;
  if (from !== '' ) return `Desde el ${formatShortDate(from)}`;
  if (to !== '') return `Hasta el ${formatShortDate(to)}`;
  return 'Todas las fechas';
}

/** The sheet to print of a team's Preparar on one map, read from `?team=&map=&from=&to=`. */
export function PrepRecord(): ReactNode {
  const query = useSearchParams();
  const teamId = query.get(PREP_QUERY.team) ?? '';
  const map = query.get(PREP_QUERY.map) ?? '';
  const fromDay = DAY_RE.test(query.get(PREP_QUERY.from) ?? '') ? (query.get(PREP_QUERY.from) ?? '') : '';
  const toDay = DAY_RE.test(query.get(PREP_QUERY.to) ?? '') ? (query.get(PREP_QUERY.to) ?? '') : '';
  const valid = UUID_RE.test(teamId) && MAP_RE.test(map);
  const [teamName, setTeamName] = useState<string | null>(null);
  const [now] = useState(() => Date.now());
  const title = teamName === null ? 'Preparar rival' : `Preparar a ${teamName}`;
  useRouteTitle(title);
  useDocumentTitle(`${title} · Equipos · ClipHub`);

  if (!valid) {
    return (
      <EmptyState
        icon={ClipboardList}
        title="Falta el rival o el mapa"
        description="Esta hoja se abre desde Equipos: en la ficha del rival, pestaña Preparar, elige el mapa y pulsa Imprimir."
        actions={
          <Button asChild size="sm">
            <Link href={TEAMS_HREF}>Ir a Equipos</Link>
          </Button>
        }
      />
    );
  }
  const { from, to } = scopeDateRange({ kind: 'range', from: fromDay, to: toDay }, now);
  return (
    <article className={`flex flex-col gap-5 print:gap-6 ${PRINT_PAPER}`}>
      <PrepBody
        query={{ teamId, map, from, to }}
        header={(prep) => {
          // The sheet is named after the team once its prep is read.
          if (teamName !== prep.team_name) queueMicrotask(() => setTeamName(prep.team_name));
          return (
            <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-3">
              <div className="flex items-center gap-3">
                <MapThumb map={prep.map} className="h-12 w-20 print:hidden" crop={1.3} />
                <div className="flex flex-col gap-0.5">
                  <h1 className="text-title font-semibold text-fg-1">
                    {prep.team_name} en {prettyMapName(prep.map)}
                  </h1>
                  <p className="text-body-sm text-fg-2 tabular-nums">
                    {periodText(fromDay, toDay)} · {scopeCountText(prep.matches, prep.round_count)}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2 print:hidden">
                <Button asChild variant="ghost" size="sm">
                  <Link href={teamPrepareHref(prep.team_id, prep.map)}>
                    <ArrowLeft aria-hidden />
                    Preparar
                  </Link>
                </Button>
                <Button type="button" size="sm" onClick={() => window.print()}>
                  <Printer aria-hidden />
                  Imprimir
                </Button>
              </div>
            </header>
          );
        }}
      />
    </article>
  );
}
