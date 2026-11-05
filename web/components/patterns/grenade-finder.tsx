'use client';

import { useMemo, useState, type ReactNode } from 'react';
import { Crosshair, X } from 'lucide-react';
import { TEAM_GRENADE_KINDS, type TeamGrenades, type TeamGrenadeKind } from '@/lib/api/grenades';
import { PRACTICE_MAX_SPOTS } from '@/lib/api/practice';
import { RADAR_LEVELS, type RadarLevel, type TacticalBuyType, type TacticalGeometry } from '@/lib/api/tactical';
import {
  clusterGrenades,
  GRENADE_KIND_LABEL,
  grenadeId,
  grenadeInBox,
  nearestCallout,
  throwTimeBins,
  type GrenadeArea,
  type WorldBox,
} from '@/lib/analysis/grenade-patterns';
import { practiceSpotsForSpots, spotLabel } from '@/lib/analysis/practice-spots';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap, levelForAltitude } from '@/lib/tactical-transform';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Toggle } from '@/components/ui/toggle';
import { GRENADE_KIND_TOKEN, GrenadeRadar } from './grenade-radar';
import { GrenadeSpotList, type LabelledSpot } from './grenade-spot-list';
import { PracticeResult, usePracticeLaunch } from './practice-launch';
import { ThrowTimeHistogram } from './throw-time-histogram';

const AREAS: readonly { value: GrenadeArea; label: string }[] = [
  { value: 'landing', label: 'Zona de caída' },
  { value: 'throw', label: 'Zona de lanzamiento' },
];

type Props = {
  map: string;
  geometry: TacticalGeometry;
  data: TeamGrenades;
  buy: TacticalBuyType | '';
  matchLabel: (matchId: string) => string;
};

/** The Pattern Finder on one map: radar with tracers, box filter, throw times and the spots list. */
export function GrenadeFinder({ map, geometry, data, buy, matchLabel }: Props): ReactNode {
  const [kinds, setKinds] = useState<ReadonlySet<TeamGrenadeKind>>(() => new Set(TEAM_GRENADE_KINDS));
  const [area, setArea] = useState<GrenadeArea>('landing');
  const [box, setBox] = useState<WorldBox | null>(null);
  const [level, setLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const [hover, setHover] = useState<string | null>(null);
  const [chosen, setChosen] = useState<readonly string[]>([]);
  const practice = usePracticeLaunch(map);
  const { calibration } = geometry;
  const multiLevel = isMultiLevelMap(calibration);

  const onLevel = useMemo(
    () => data.grenades.filter((grenade) => (buy === '' || grenade.buy === buy) && levelForAltitude(calibration, grenade.landing[2]) === level),
    [data.grenades, buy, calibration, level],
  );
  const counts = useMemo(() => {
    const out = new Map<TeamGrenadeKind, number>();
    for (const grenade of onLevel) out.set(grenade.kind, (out.get(grenade.kind) ?? 0) + 1);
    return out;
  }, [onLevel]);
  const drawn = useMemo(() => onLevel.filter((grenade) => kinds.has(grenade.kind)), [onLevel, kinds]);
  const selected = useMemo(
    () => (box === null ? null : new Set(drawn.filter((grenade) => grenadeInBox(grenade, box, area)).map(grenadeId))),
    [drawn, box, area],
  );
  const matching = useMemo(() => (selected === null ? drawn : drawn.filter((grenade) => selected.has(grenadeId(grenade)))), [drawn, selected]);

  const spots = useMemo((): LabelledSpot[] => {
    const callouts = geometry.callouts.filter((callout) => callout.level === level);
    return clusterGrenades(matching).map((spot) => ({
      spot,
      label: spotLabel(spot.kind, nearestCallout(callouts, spot.landing), spot.origin === null ? null : nearestCallout(callouts, spot.origin)),
    }));
  }, [matching, geometry.callouts, level]);
  const spotOf = useMemo(() => {
    const out = new Map<string, string>();
    for (const { spot } of spots) for (const grenade of spot.grenades) out.set(grenadeId(grenade), spot.key);
    return out;
  }, [spots]);
  const bins = useMemo(() => throwTimeBins(matching), [matching]);
  const missingOrigins = matching.filter((grenade) => grenade.origin === null).length;
  const chosenSpots = spots.filter(({ spot }) => chosen.includes(spot.key));

  const toggleKind = (kind: TeamGrenadeKind): void =>
    setKinds((current) => {
      const next = new Set(current);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });
  const choose = (key: string): void =>
    setChosen((current) => (current.includes(key) ? current.filter((entry) => entry !== key) : [...current, key].slice(0, PRACTICE_MAX_SPOTS)));

  return (
    <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1.2fr)_minmax(20rem,1fr)]">
      <section aria-label="Radar" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <div role="group" aria-label="Tipos de granada" className="flex flex-wrap gap-1.5">
            {TEAM_GRENADE_KINDS.map((kind) => (
              <Toggle key={kind} variant="filter" size="sm" pressed={kinds.has(kind)} onPressedChange={() => toggleKind(kind)}>
                <span aria-hidden className="size-2.5 rounded-full" style={{ backgroundColor: `var(${GRENADE_KIND_TOKEN[kind]})` }} />
                {GRENADE_KIND_LABEL[kind]}
                <span className="tabular-nums opacity-80">{counts.get(kind) ?? 0}</span>
              </Toggle>
            ))}
          </div>
          {multiLevel ? (
            <Segmented
              label="Piso"
              value={level}
              options={[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] }))}
              onChange={(next) => {
                setLevel(next);
                setBox(null);
              }}
            />
          ) : null}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Segmented label="La zona filtra por" value={area} options={AREAS} onChange={setArea} />
          {box === null ? (
            <span className="text-body-sm text-fg-3">Arrastra sobre el radar para quedarte con una zona.</span>
          ) : (
            <Button type="button" variant="ghost" size="xs" onClick={() => setBox(null)}>
              <X aria-hidden />
              Quitar zona
            </Button>
          )}
        </div>
        <GrenadeRadar
          geometry={geometry}
          level={level}
          grenades={drawn}
          spotOf={spotOf}
          selected={selected}
          highlight={hover}
          box={box}
          onBox={setBox}
          onHover={setHover}
        />
        <ThrowTimeHistogram bins={bins} />
      </section>

      <section aria-labelledby="granadas-posiciones" className="flex flex-col gap-3">
        <header className="flex flex-wrap items-center justify-between gap-2">
          <h2 id="granadas-posiciones" className="text-body-lg font-semibold text-fg-1">
            {spots.length} {spots.length === 1 ? 'posición' : 'posiciones'}
            <span className="ml-2 text-body-sm font-normal text-fg-2 tabular-nums">
              {matching.length} {matching.length === 1 ? 'lanzamiento' : 'lanzamientos'}
            </span>
          </h2>
          <Button
            type="button"
            size="sm"
            disabled={chosenSpots.length === 0 || practice.state.kind === 'busy'}
            onClick={() => practice.launch(practiceSpotsForSpots(chosenSpots))}
          >
            <Crosshair aria-hidden />
            {chosenSpots.length === 0 ? 'Practicar selección' : `Practicar selección (${chosenSpots.length})`}
          </Button>
        </header>
        <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
        {chosenSpots.length >= PRACTICE_MAX_SPOTS ? (
          <p className="text-body-sm text-fg-2">Caben {PRACTICE_MAX_SPOTS} posiciones por práctica.</p>
        ) : null}
        {missingOrigins > 0 ? (
          <p className="text-body-sm text-fg-2">
            {missingOrigins} {missingOrigins === 1 ? 'granada no tiene' : 'granadas no tienen'} posición de lanzamiento: son de
            demos analizadas con una versión anterior. Studio las vuelve a analizar al arrancar.
          </p>
        ) : null}
        {spots.length === 0 ? (
          <p className="text-body-sm text-fg-2">
            {box === null ? 'No hay granadas con este filtro.' : 'Ninguna granada en esta zona.'}
            {box === null ? null : (
              <Button type="button" variant="link" size="xs" onClick={() => setBox(null)}>
                Quitar zona
              </Button>
            )}
          </p>
        ) : (
          <GrenadeSpotList
            spots={spots}
            highlight={hover}
            onHover={setHover}
            chosen={new Set(chosen)}
            onChoose={choose}
            full={chosenSpots.length >= PRACTICE_MAX_SPOTS}
            busy={practice.state.kind === 'busy'}
            onPractice={practice.launch}
            matchLabel={matchLabel}
          />
        )}
      </section>
    </div>
  );
}
