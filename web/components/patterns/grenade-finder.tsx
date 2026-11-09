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
import { EmptySample } from '@/components/analysis/empty-sample';
import { Button } from '@/components/ui/button';
import { useCalloutNames } from '@/hooks/use-callout-names';
import { Segmented } from '@/components/ui/segmented';
import { Toggle } from '@/components/ui/toggle';
import { GRENADE_KIND_TOKEN, GrenadeRadar } from './grenade-radar';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { grenadeIcon } from '@/lib/analysis/icons';
import { GrenadeSpotList, type LabelledSpot } from './grenade-spot-list';
import { PracticeResult, usePracticeLaunch } from './practice-launch';
import { TramoBar, useTramo } from '@/components/replay/tramo-bar';
import type { TramoControl } from '@/components/replay/tramo-bar';
import { ROUND_CLOCK_SECONDS, inTramo } from '@/lib/replay/tramo';

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
  /** The owner's tramo; without it the finder keeps its own. */
  tramo?: TramoControl;
  /** When the bomb went down in a round, in seconds after freeze end; needed for the tramo from the plant. */
  plantSeconds?: (matchId: string, round: number) => number | null;
};

/** The Pattern Finder on one map: radar with tracers, box filter, throw times and the spots list. */
export function GrenadeFinder({ map, geometry, data, buy, matchLabel, tramo: tramoControl, plantSeconds }: Props): ReactNode {
  const { tramo, onChange: setTramo } = useTramo(tramoControl);
  const [kinds, setKinds] = useState<ReadonlySet<TeamGrenadeKind>>(() => new Set(TEAM_GRENADE_KINDS));
  const [area, setArea] = useState<GrenadeArea>('landing');
  const [box, setBox] = useState<WorldBox | null>(null);
  const [boxLevel, setBoxLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const [floor, setFloor] = useState<RadarLevel | 'both'>('both');
  const [hover, setHover] = useState<string | null>(null);
  const [chosen, setChosen] = useState<readonly string[]>([]);
  const practice = usePracticeLaunch(map);
  const calloutLabel = useCalloutNames([map]);
  const { calibration } = geometry;
  const multiLevel = isMultiLevelMap(calibration);
  // On Nuke and Vertigo both floors sit side by side by default; a grenade belongs to the floor it lands on.
  const levels = useMemo<RadarLevel[]>(
    () => (multiLevel && floor === 'both' ? [RADAR_LEVELS.default, RADAR_LEVELS.lower] : [floor === 'both' ? RADAR_LEVELS.default : floor]),
    [multiLevel, floor],
  );

  const onLevel = useMemo(
    () => data.grenades.filter((grenade) => (buy === '' || grenade.buy === buy) && levels.includes(levelForAltitude(calibration, grenade.landing[2]))),
    [data.grenades, buy, calibration, levels],
  );
  const counts = useMemo(() => {
    const out = new Map<TeamGrenadeKind, number>();
    for (const grenade of onLevel) out.set(grenade.kind, (out.get(grenade.kind) ?? 0) + 1);
    return out;
  }, [onLevel]);
  const ofKinds = useMemo(() => onLevel.filter((grenade) => kinds.has(grenade.kind)), [onLevel, kinds]);
  // The tramo counts the throw by when the grenade went off, the moment the histogram bins too.
  const drawn = useMemo(
    () => ofKinds.filter((grenade) => inTramo(tramo, grenade.seconds, plantSeconds?.(grenade.match_id, grenade.round) ?? null)),
    [ofKinds, tramo, plantSeconds],
  );
  const selected = useMemo(
    () => (box === null ? null : new Set(drawn.filter((grenade) => grenadeInBox({ grenade, box, area, floor: { calibration, level: boxLevel } })).map(grenadeId))),
    [drawn, box, area, calibration, boxLevel],
  );
  const matching = useMemo(() => (selected === null ? drawn : drawn.filter((grenade) => selected.has(grenadeId(grenade)))), [drawn, selected]);

  const spots = useMemo((): LabelledSpot[] => {
    const place = (token: string): string => calloutLabel(map, token);
    return clusterGrenades(matching).map((spot) => {
      const level = levelForAltitude(calibration, spot.grenades[0]?.landing[2] ?? 0);
      const callouts = geometry.callouts.filter((callout) => callout.level === level);
      const label = spotLabel(
        spot.kind,
        { landing: nearestCallout(callouts, spot.landing), origin: spot.origin === null ? null : nearestCallout(callouts, spot.origin) },
        place,
      );
      return { spot, label };
    });
  }, [matching, geometry.callouts, calibration, calloutLabel, map]);
  const spotOf = useMemo(() => {
    const out = new Map<string, string>();
    for (const { spot } of spots) for (const grenade of spot.grenades) out.set(grenadeId(grenade), spot.key);
    return out;
  }, [spots]);
  // Every throw of the kinds in view, so the bars show what a narrower tramo would keep.
  const bins = useMemo(() => throwTimeBins(ofKinds, 5, ROUND_CLOCK_SECONDS), [ofKinds]);
  const tallest = bins.reduce((top, bin) => Math.max(top, bin.count), 0);
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
    <div className="grid items-start gap-3 lg:grid-cols-[minmax(0,1.4fr)_minmax(22rem,1fr)]">
      <section aria-label="Radar" className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <div role="group" aria-label="Tipos de granada" className="flex flex-wrap gap-1.5">
            {TEAM_GRENADE_KINDS.map((kind) => (
              <Toggle key={kind} variant="filter" size="sm" pressed={kinds.has(kind)} onPressedChange={() => toggleKind(kind)}>
                <span aria-hidden className="flex" style={{ color: `var(${GRENADE_KIND_TOKEN[kind]})` }}>
                <EquipmentIcon name={grenadeIcon(kind)} height={14} />
              </span>
                {GRENADE_KIND_LABEL[kind]}
                <span className="tabular-nums opacity-80">{counts.get(kind) ?? 0}</span>
              </Toggle>
            ))}
          </div>
          {multiLevel ? (
            <Segmented<RadarLevel | 'both'>
              label="Piso"
              value={floor}
              options={[
                { value: 'both', label: 'Los dos' },
                ...[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] })),
              ]}
              onChange={(next) => {
                setFloor(next);
                setBox(null);
              }}
            />
          ) : null}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Segmented label="La zona filtra por" value={area} options={AREAS} onChange={setArea} />
          {box === null ? (
            <span className="text-body-sm text-fg-3">Arrastra sobre el radar para quedarte con una zona; con zoom, Mayús + arrastrar.</span>
          ) : (
            <Button type="button" variant="ghost" size="xs" onClick={() => setBox(null)}>
              <X aria-hidden />
              Quitar zona
            </Button>
          )}
        </div>
        <GrenadeRadar
          geometry={geometry}
          levels={levels}
          grenades={drawn}
          spotOf={spotOf}
          selected={selected}
          highlight={hover}
          box={box === null ? null : { box, level: boxLevel }}
          onBox={(next) => {
            setBox(next?.box ?? null);
            if (next !== null) setBoxLevel(next.level);
          }}
          onHover={setHover}
        />
        <TramoBar
          tramo={tramo}
          onChange={setTramo}
          marks={
            <div className="absolute inset-0 flex items-end gap-px" role="img" aria-label="Cuándo se lanzan las granadas a lo largo de la ronda">
              {bins.map((bin) => (
                <span key={bin.start} className="flex-1 rounded-t-xs bg-fg-3" style={{ height: tallest === 0 ? 0 : `${(bin.count / tallest) * 100}%` }} />
              ))}
            </div>
          }
        />
      </section>

      <section aria-labelledby="granadas-posiciones" className="flex flex-col gap-2 lg:sticky lg:top-[calc(var(--shell-strip-height)+3.5rem)] lg:max-h-[calc(100dvh-var(--shell-strip-height)-4.5rem)] lg:overflow-y-auto">
        <header className="flex flex-wrap items-center justify-between gap-2">
          <h2 id="granadas-posiciones" className="text-body font-semibold text-fg-1">
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
          <EmptySample hiddenLocally={data.grenades.length > 0}>
            <p className="text-body-sm text-fg-2">
              {box === null ? 'No hay granadas con este filtro.' : 'Ninguna granada en esta zona.'}
              {box === null ? null : (
                <Button type="button" variant="link" size="xs" onClick={() => setBox(null)}>
                  Quitar zona
                </Button>
              )}
            </p>
          </EmptySample>
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
