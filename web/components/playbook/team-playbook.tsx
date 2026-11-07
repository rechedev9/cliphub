'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { getTeamPlaybook } from '@/lib/api/library';
import type { PlaybookCombo, TeamPlaybook as Playbook } from '@/lib/api/playbook';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { prettyMapName } from '@/lib/format';
import { libraryMatchesText, normsCategories } from '@/lib/analysis/library-pool';
import {
  EMPTY_COMBO,
  comboFromQuery,
  comboIsEmpty,
  comboSummary,
  narrowPlaybook,
  positionsNote,
  toggleTactic,
  writeComboQuery,
  type ComboMode,
} from '@/lib/analysis/playbook-combo';
import { useLibraryNorms } from '@/hooks/use-library-norms';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { MapPinned, Swords } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { CategoryGrid, SiteBars, SkippedNote, TempoChart } from './playbook-summary';
import { ComboBar, TacticBoard, comboLabeler } from './tactic-board';

function Section({ title, children, className }: { title: string; children: ReactNode; className?: string }): ReactNode {
  return (
    <section aria-label={title} className={className ?? 'studio-panel flex flex-col gap-3 p-4'}>
      <h2 className="text-body font-semibold text-fg-1">{title}</h2>
      {children}
    </section>
  );
}

/** Playbook reads one map and one side: ask for whichever is missing, with the choices at hand. */
function ChooseScope({ scope, maps, onScopeChange }: { scope: AnalysisScope; maps: readonly string[]; onScopeChange: (patch: Partial<AnalysisScope>) => void }): ReactNode {
  if (scope.map === '') {
    return (
      <EmptyState
        icon={MapPinned}
        title="Elige un mapa"
        description={scope.side === '' ? 'El playbook se lee mapa a mapa y lado a lado.' : 'El playbook se lee mapa a mapa.'}
        actions={
          maps.length === 0 ? undefined : (
            <div className="flex flex-wrap gap-2">
              {maps.map((entry) => (
                <Button key={entry} type="button" variant="outline" size="sm" onClick={() => onScopeChange({ map: entry })}>
                  {prettyMapName(entry)}
                </Button>
              ))}
            </div>
          )
        }
      />
    );
  }
  return (
    <EmptyState
      icon={Swords}
      title="Elige un lado"
      description={`Cómo juegan ${prettyMapName(scope.map)} de CT o de T.`}
      actions={
        <div className="flex flex-wrap gap-2">
          {Object.values(TACTICAL_SIDES).map((entry) => (
            <Button key={entry} type="button" variant="outline" size="sm" onClick={() => onScopeChange({ side: entry })}>
              {entry}
            </Button>
          ))}
        </div>
      }
    />
  );
}

/** The team's playbook on one side of one map: round types, tempo, sites and the zones it controls. */
export function TeamPlaybook({
  teamId,
  scope,
  maps,
  onScopeChange,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The maps the team has played, offered when none is chosen. */
  maps: readonly string[];
  onScopeChange: (patch: Partial<AnalysisScope>) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [playbook, setPlaybook] = useState<Playbook | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [now] = useState(() => Date.now());
  const router = useRouter();
  const pathname = usePathname();
  const query = useSearchParams().toString();
  const known = useMemo(() => new Set(playbook?.tactics.map((tactic) => tactic.key) ?? []), [playbook]);
  const combo = useMemo(() => comboFromQuery(query, known), [query, known]);
  const view = useMemo(() => (playbook === null ? null : narrowPlaybook(playbook, combo)), [playbook, combo]);
  const labelOf = useMemo(() => (playbook === null ? null : comboLabeler(playbook)), [playbook]);
  const setCombo = useCallback(
    (next: PlaybookCombo) => {
      const search = writeComboQuery(next, query);
      router.replace(search === '' ? pathname : `${pathname}?${search}`, { scroll: false });
    },
    [router, pathname, query],
  );
  const toggle = useCallback((key: string, mode: ComboMode) => setCombo(toggleTactic(combo, key, mode)), [setCombo, combo]);
  const clear = useCallback(() => setCombo(EMPTY_COMBO), [setCombo]);
  const { map, side, opponent } = scope;
  const { from, to } = scopeDateRange(scope.period, now);
  const ready = map !== '' && side !== '';
  const norms = useLibraryNorms(ready ? map : '');
  const reference = ready && norms !== null && norms !== 'failed' ? normsCategories(norms, side) : null;

  useEffect(() => {
    if (map === '' || side === '') return undefined;
    let active = true;
    setPlaybook(null);
    setFailed(false);
    onCounts(null);
    getTeamPlaybook(teamId, { map, side, from, to, opponent_ids: opponent === '' ? [] : [opponent] })
      .then((next) => {
        if (!active) return;
        setPlaybook(next);
        onCounts({ maps: next.matches, rounds: next.round_count });
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, opponent, from, to, attempt, onCounts]);

  if (!ready) return <ChooseScope scope={scope} maps={maps} onScopeChange={onScopeChange} />;
  if (failed) {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo leer el playbook. Comprueba que ClipHub Studio sigue abierto y{' '}
        <button type="button" className="underline underline-offset-4" onClick={() => setAttempt((n) => n + 1)}>
          vuelve a intentarlo
        </button>
        .
      </p>
    );
  }
  if (playbook === null || view === null || labelOf === null) {
    return (
      <div role="status" aria-label="Leyendo el playbook" className="flex flex-col gap-4">
        <Skeleton className="h-36 w-full" />
        <div className="grid gap-4 lg:grid-cols-2">
          <Skeleton className="h-52 w-full" />
          <Skeleton className="h-52 w-full" />
        </div>
        <Skeleton className="h-80 w-full" />
      </div>
    );
  }
  if (playbook.round_count === 0) {
    return (
      <p className="text-body text-fg-2">
        No hay rondas de {side} en {prettyMapName(map)} con este filtro. Prueba con otro periodo o rival.
      </p>
    );
  }

  const narrowed = !comboIsEmpty(combo);
  const note = positionsNote(view);
  return (
    <div className="flex flex-col gap-4">
      <section aria-label="Combinación" className="studio-panel flex flex-col gap-2 p-4">
        <h2 className="text-body font-semibold text-fg-1">Combina tácticas</h2>
        <ComboBar teamId={teamId} view={view} labelOf={labelOf} onToggle={toggle} onClear={clear} />
      </section>
      <Section title="Tipos de ronda" className="flex flex-col gap-3">
        <p className="text-body-sm text-fg-2">
          {narrowed
            ? comboSummary(view, playbook.round_count)
            : `Gana el ${Math.round(playbook.win_rate.pct)} % de sus ${playbook.round_count} rondas.`}{' '}
          Pulsa un tipo para ver sus rondas.
          {norms !== null && norms !== 'failed' && !narrowed ? ` ▲▼ frente a los dos equipos de ${libraryMatchesText(norms.matches)} en ${prettyMapName(map)}, de ${side}.` : ''}
        </p>
        <CategoryGrid playbook={view} reference={narrowed ? null : reference} />
      </Section>
      <div className="grid gap-4 lg:grid-cols-2">
        <Section title="Ritmo">
          <TempoChart playbook={view} rounds={view.rounds} />
        </Section>
        <Section title="Sitios">
          <SiteBars playbook={view} />
        </Section>
      </div>
      <Section title="Tácticas">
        {note === '' ? null : <p className="text-meta text-fg-3">{note}</p>}
        <TacticBoard teamId={teamId} playbook={playbook} view={view} onToggle={toggle} />
      </Section>
      <SkippedNote skipped={playbook.skipped} />
    </div>
  );
}
