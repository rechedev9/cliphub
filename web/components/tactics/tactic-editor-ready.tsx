'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { Swords } from 'lucide-react';
import type { PlaybookCombo } from '@/lib/api/playbook';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import { lineupSteamIds } from '@/lib/analysis/lineup';
import { EMPTY_COMBO, comboFromQuery, comboIsEmpty, narrowPlaybook, toggleTactic, writeComboQuery, type ComboMode } from '@/lib/analysis/playbook-combo';
import type { ScopeField } from '@/lib/analysis/scope';
import { chosenZones, keptText } from '@/lib/analysis/tactic-editor';
import { winPct } from '@/lib/analysis/playbook';
import { prettyMapName } from '@/lib/format';
import { NewPlayDialog } from '@/components/libreta/new-play-dialog';
import { ComboBar, comboLabeler } from '@/components/playbook/tactic-board';
import { SkippedNote } from '@/components/playbook/playbook-summary';
import { ZoneMap } from '@/components/playbook/zone-map';
import { SlashTitle } from '@/components/records/document-page';
import { TeamScopeFilter, type TeamScopeFilterProps } from '@/components/analysis/team-scope-filter';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { useTeamMapCounts } from '@/hooks/use-team-map-counts';
import { useTeamPlaybook } from '@/hooks/use-team-playbook';
import { useTeamScopeControl } from '@/hooks/use-team-scope-control';
import { useTeamTactics, type TeamAnalysis } from '@/hooks/use-team-analysis';
import { ConditionPanels } from './condition-panels';
import { DetectedPlays } from './detected-plays';

/** The controls the editor reads: the playbook is one map and one side of the team. */
const EDITOR_FIELDS: readonly ScopeField[] = ['lineup', 'map', 'side', 'period', 'opponent'];

type RightTab = 'combinacion' | 'repetidas';

function ChooseSide({ map, onSide }: { map: string; onSide: (side: 'CT' | 'T') => void }): ReactNode {
  return (
    <div className="flex flex-col items-start gap-3 py-6">
      <p className="text-body text-fg-2">Elige el lado: {prettyMapName(map)} de CT o de T.</p>
      <div className="flex gap-2">
        {Object.values(TACTICAL_SIDES).map((side) => (
          <Button key={side} type="button" variant="outline" size="sm" onClick={() => onSide(side)}>
            {side}
          </Button>
        ))}
      </div>
    </div>
  );
}

/**
 * The Editor de tácticas of one team: the filter stack and the condition panels on the left, the map
 * with its zones in the centre, and the combination with its two ways into the Libreta on the right.
 * It reads the team playbook, so a combination here is the same one the Playbook tab builds.
 */
export function TacticEditorReady({ analysis, teams, onTeam }: { analysis: TeamAnalysis; teams: TeamScopeFilterProps['teams']; onTeam: (id: string) => void }): ReactNode {
  const { team, matches, lineups } = analysis;
  const tactics = useTeamTactics(team.id);
  const control = useTeamScopeControl({ teamId: team.id, tactics, matches, defaultMap: true });
  const { scope } = control;
  const lineupKey = lineupSteamIds(scope.lineup, lineups).join(',');
  const lineup = useMemo(() => (lineupKey === '' ? [] : lineupKey.split(',')), [lineupKey]);
  const mapCounts = useTeamMapCounts({ teamId: team.id, scope, fields: EDITOR_FIELDS, lineup, enabled: true });
  const { playbook, failed, retry } = useTeamPlaybook({ teamId: team.id, scope, lineup });
  const router = useRouter();
  const pathname = usePathname();
  const query = useSearchParams().toString();
  const [tab, setTab] = useState<RightTab>('combinacion');
  const [creating, setCreating] = useState(false);

  const known = useMemo(() => new Set(playbook?.tactics.map((tactic) => tactic.key) ?? []), [playbook]);
  const combo = useMemo(() => comboFromQuery(query, known), [query, known]);
  const view = useMemo(() => (playbook === null ? null : narrowPlaybook(playbook, combo)), [playbook, combo]);
  const labelOf = useMemo(() => (playbook === null ? null : comboLabeler(playbook)), [playbook]);
  const setCombo = useCallback(
    (next: PlaybookCombo) => {
      const search = writeComboQuery(next, query);
      router.replace(search === '' ? pathname : `${pathname}?${search}`, { scroll: false });
    },
    [pathname, query, router],
  );
  const toggle = useCallback((key: string, mode: ComboMode) => setCombo(toggleTactic(combo, key, mode)), [combo, setCombo]);
  const clear = useCallback(() => setCombo(EMPTY_COMBO), [setCombo]);
  const toggleZone = useCallback(
    (zone: string) => {
      if (known.has(`zone:${zone}`)) toggle(`zone:${zone}`, 'with');
    },
    [known, toggle],
  );
  const labels = useMemo(() => new Map((playbook?.zones ?? []).map((zone) => [zone.zone, zone.label])), [playbook]);
  const counts = playbook === null ? null : { maps: playbook.matches, rounds: playbook.round_count };

  const stack = (
    <div className="rounded-sm border border-border bg-surface-1 p-2">
      <TeamScopeFilter
        form="stack"
        control={control}
        analysis={analysis}
        teams={teams}
        tactics={tactics}
        fields={EDITOR_FIELDS}
        mapRequired
        mapCounts={mapCounts}
        counts={scope.map === '' || scope.side === '' ? undefined : counts}
        onTeam={onTeam}
        className="w-full"
      />
    </div>
  );

  let centre: ReactNode;
  if (scope.map === '') {
    centre = <p className="py-6 text-body text-fg-2">Este equipo aún no tiene partidas analizadas con las que armar una táctica.</p>;
  } else if (scope.side === '') {
    centre = <ChooseSide map={scope.map} onSide={(side) => control.changeScope({ side })} />;
  } else if (failed) {
    centre = (
      <p role="alert" className="py-6 text-body-sm text-destructive">
        No se pudo leer el playbook. Comprueba que ClipHub Studio sigue abierto y{' '}
        <button type="button" className="underline underline-offset-4" onClick={retry}>
          vuelve a intentarlo
        </button>
        .
      </p>
    );
  } else if (playbook === null || view === null) {
    centre = <Skeleton role="status" aria-label="Leyendo el playbook" className="aspect-square w-full max-w-[44rem]" />;
  } else if (playbook.round_count === 0) {
    centre = (
      <p className="py-6 text-body text-fg-2">
        No hay rondas de {scope.side} en {prettyMapName(scope.map)} con este filtro. Prueba con otro periodo o rival.
      </p>
    );
  } else {
    const won = winPct(view.rounds);
    centre = (
      <div className="flex flex-col gap-3">
        <p aria-live="polite" className="font-num text-body-sm text-fg-2 tabular-nums">
          {keptText({ kept: view.round_count, total: playbook.round_count })}
          {won === null ? '' : `, gana el ${won} %`}
        </p>
        <ZoneMap
          teamId={team.id}
          map={playbook.map}
          zones={playbook.zones.map((zone) => zone.zone)}
          labels={labels}
          selected={chosenZones(view.combo)}
          onZone={toggleZone}
          className="max-w-[44rem]"
        />
        <p className="text-meta text-fg-3">Pulsa una zona de la que controlen para quedarte con las rondas que la sujetan.</p>
        <SkippedNote skipped={playbook.skipped} />
      </div>
    );
  }

  const ready = playbook !== null && view !== null && labelOf !== null && playbook.round_count > 0;
  let side: ReactNode;
  if (tab === 'repetidas') {
    side =
      scope.map === '' || scope.side === '' ? (
        <p className="text-body-sm text-fg-2">Elige un mapa y un lado para buscar las jugadas que repite el equipo.</p>
      ) : (
        <DetectedPlays teamId={team.id} teamName={team.name} scope={scope} lineup={lineup} />
      );
  } else if (ready) {
    side = (
      <>
        <ComboBar teamId={team.id} view={view} labelOf={labelOf} onToggle={toggle} onClear={clear} />
        {comboIsEmpty(combo) ? null : (
          <p className="text-meta text-fg-3">Al guardar, la jugada queda en la Libreta con la combinación en las notas y sus primeras rondas de ejemplo.</p>
        )}
      </>
    );
  } else {
    side = <p className="text-body-sm text-fg-2">Elige un equipo, un mapa y un lado y añade condiciones para armar la táctica.</p>;
  }

  return (
    <section aria-label="Editor de tácticas" className="grid items-start gap-3 min-[1180px]:grid-cols-[18rem_minmax(0,1fr)_20rem]">
      <div className="flex min-w-0 flex-col gap-2 min-[1180px]:max-h-[calc(100dvh-var(--shell-strip-height)-2rem)] min-[1180px]:overflow-y-auto min-[1180px]:[scrollbar-width:thin]">
        {stack}
        {ready ? <ConditionPanels playbook={playbook} view={view} onToggle={toggle} /> : null}
      </div>

      <div className="flex min-w-0 flex-col gap-3">
        <SlashTitle as="h1">Editor de tácticas</SlashTitle>
        {centre}
      </div>

      <aside aria-label="Táctica" className="flex min-w-0 flex-col gap-3 rounded-sm border border-border bg-surface-1 p-3">
        <Segmented
          label="Qué ver"
          value={tab}
          options={[
            { value: 'combinacion', label: 'Combinación' },
            { value: 'repetidas', label: 'Jugadas repetidas' },
          ]}
          onChange={setTab}
        />
        {side}
        <div className="border-t border-border-subtle pt-3">
          <Button type="button" variant="outline" size="sm" disabled={scope.map === '' || scope.side === ''} onClick={() => setCreating(true)}>
            <Swords aria-hidden />
            Crear jugada a mano
          </Button>
        </div>
      </aside>
      {creating ? (
        <NewPlayDialog
          preset={{ map: scope.map, side: scope.side === 'CT' || scope.side === 'T' ? scope.side : undefined, teamId: team.id }}
          knownMaps={[...new Set(matches.map((match) => match.map))].sort()}
          onClose={() => setCreating(false)}
        />
      ) : null}
    </section>
  );
}
