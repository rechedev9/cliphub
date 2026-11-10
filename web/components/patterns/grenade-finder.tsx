'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { BookPlus, Crosshair, Layers, X } from 'lucide-react';
import { TEAM_GRENADE_KINDS, type TeamGrenade, type TeamGrenades, type TeamGrenadeKind } from '@/lib/api/grenades';
import type { LineupThrow } from '@/lib/api/lineups';
import { PRACTICE_MAX_SPOTS } from '@/lib/api/practice';
import { RADAR_LEVELS, type RadarLevel, type TacticalBuyType, type TacticalGeometry } from '@/lib/api/tactical';
import { denominatorText, followUpOf, roundKey, roundRows, roundsSide, thrownRounds } from '@/lib/analysis/grenade-follow-up';
import {
  clockBins,
  GRENADE_KIND_LABEL,
  grenadeId,
  grenadeInBox,
  groupSpots,
  spotReplayRefs,
  throwersOf,
  type GrenadeArea,
  type WorldBox,
} from '@/lib/analysis/grenade-patterns';
import { playFromGrenadeSpots } from '@/lib/analysis/grenade-play';
import { grenadeIcon } from '@/lib/analysis/icons';
import { practiceSpotsForSpots, spotLabel } from '@/lib/analysis/practice-spots';
import { REPLAY_MAX_REFS, superposedReplayHref } from '@/lib/analysis/replay-link';
import { inTramo } from '@/lib/replay/tramo';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap, levelForAltitude } from '@/lib/tactical-transform';
import { useCalloutNames } from '@/hooks/use-callout-names';
import { EmptySample } from '@/components/analysis/empty-sample';
import { SaveLineupDialog } from '@/components/libreta/save-lineup-dialog';
import { TramoBar, useTramo, type TramoControl } from '@/components/replay/tramo-bar';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Segmented } from '@/components/ui/segmented';
import { Toggle } from '@/components/ui/toggle';
import { GrenadeAfterPlant, GrenadeClockMarks } from './grenade-clock';
import { GrenadePlayerPicker, playerToken } from './grenade-players';
import { GRENADE_KIND_TOKEN, GrenadeRadar } from './grenade-radar';
import { GrenadeRoundList } from './grenade-round-list';
import { GrenadeSpotList, type LabelledSpot } from './grenade-spot-list';
import { GrenadeSpotPanel } from './grenade-spot-panel';
import { PracticeResult, usePracticeLaunch } from './practice-launch';
import { SaveGrenadePlayDialog } from './save-grenade-play';

const AREAS: readonly { value: GrenadeArea; label: string }[] = [
  { value: 'landing', label: 'Dónde cae' },
  { value: 'throw', label: 'Desde dónde se tira' },
];

type ResultsView = 'spots' | 'rounds';
const RESULTS: readonly { value: ResultsView; label: string }[] = [
  { value: 'spots', label: 'Por posición' },
  { value: 'rounds', label: 'Por ronda' },
];

type Props = {
  map: string;
  geometry: TacticalGeometry;
  data: TeamGrenades;
  buy: TacticalBuyType | '';
  matchLabel: (matchId: string) => string;
  /** The owner's tramo; without it the finder keeps its own. */
  tramo?: TramoControl;
  /** When the bomb went down in a round, in seconds after freeze end; by default the server's round list says. */
  plantSeconds?: (matchId: string, round: number) => number | null;
  /** The players whose throws count when the page keeps them (the Replay's focus); without it the finder shows its own picker. */
  players?: readonly string[];
  /** "Rivales de <equipo>", where the page can read the rivals' utility in the same rounds. */
  rivals?: { on: boolean; label: string; onChange: (on: boolean) => void };
  /** The team the grenades belong to, for Guardar como jugada; null when there is none to save them to. */
  teamId: string | null;
};

/** The Pattern Finder on one map: radar with tracers, box filter, players, the round clock and the spots or rounds. */
export function GrenadeFinder({ map, geometry, data, buy, matchLabel, tramo: tramoControl, plantSeconds, players: pagePlayers, rivals, teamId }: Props): ReactNode {
  const { tramo, onChange: setTramo } = useTramo(tramoControl);
  const [kinds, setKinds] = useState<ReadonlySet<TeamGrenadeKind>>(() => new Set(TEAM_GRENADE_KINDS));
  const [area, setArea] = useState<GrenadeArea>('landing');
  const [box, setBox] = useState<WorldBox | null>(null);
  const [boxLevel, setBoxLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const [floor, setFloor] = useState<RadarLevel | 'both'>('both');
  const [hover, setHover] = useState<string | null>(null);
  const [chosen, setChosen] = useState<readonly string[]>([]);
  const [ownPlayers, setOwnPlayers] = useState<readonly string[]>([]);
  const [open, setOpen] = useState<string | null>(null);
  const [results, setResults] = useState<ResultsView>('spots');
  const [invert, setInvert] = useState(false);
  const [saving, setSaving] = useState<{ from: LineupThrow; label: string } | null>(null);
  const [savingPlay, setSavingPlay] = useState<readonly LabelledSpot[] | null>(null);
  const practice = usePracticeLaunch(map);
  const calloutLabel = useCalloutNames([map]);
  const place = useCallback((token: string) => calloutLabel(map, token), [calloutLabel, map]);
  const picked = pagePlayers ?? ownPlayers;
  const { calibration } = geometry;
  const multiLevel = isMultiLevelMap(calibration);
  // On Nuke and Vertigo both floors sit side by side by default; a grenade belongs to the floor it lands on.
  const levels = useMemo<RadarLevel[]>(
    () => (multiLevel && floor === 'both' ? [RADAR_LEVELS.default, RADAR_LEVELS.lower] : [floor === 'both' ? RADAR_LEVELS.default : floor]),
    [multiLevel, floor],
  );

  // The scope's rounds after the bar's buy, the team's own also with the rivals' grenades: the denominator of every count here.
  const scopeRounds = useMemo(() => data.round_list.filter((round) => buy === '' || round.team_buy === buy), [data.round_list, buy]);
  const total = data.round_list.length === 0 && buy === '' ? data.rounds : scopeRounds.length;
  const side = roundsSide(scopeRounds);
  const plants = useMemo(() => new Map(data.round_list.map((round) => [roundKey(round.match_id, round.round), round.plant_seconds])), [data.round_list]);
  const plantOf = useCallback(
    (matchId: string, round: number) => plantSeconds?.(matchId, round) ?? plants.get(roundKey(matchId, round)) ?? null,
    [plantSeconds, plants],
  );

  const ofBuy = useMemo(() => {
    if (buy === '') return data.grenades;
    if (data.round_list.length === 0) return data.grenades.filter((grenade) => grenade.buy === buy);
    const kept = new Set(scopeRounds.map((round) => roundKey(round.match_id, round.round)));
    return data.grenades.filter((grenade) => kept.has(roundKey(grenade.match_id, grenade.round)));
  }, [data.grenades, data.round_list.length, scopeRounds, buy]);
  // The order (and so the colours) comes from every throw, so a filter never moves a player's colour.
  const order = useMemo(() => throwersOf(data.grenades), [data.grenades]);
  const tokens = useMemo(() => new Map(order.map((thrower, index) => [thrower.steamid64, playerToken(index)])), [order]);
  const throwers = useMemo(() => {
    const counts = new Map(throwersOf(ofBuy).map((thrower) => [thrower.steamid64, thrower.throws]));
    return order.map((thrower) => ({ ...thrower, throws: counts.get(thrower.steamid64) ?? 0 }));
  }, [order, ofBuy]);
  const names = useMemo(() => new Map(order.map((thrower) => [thrower.steamid64, thrower.name])), [order]);
  const onLevel = useMemo(
    () =>
      ofBuy.filter(
        (grenade) => levels.includes(levelForAltitude(calibration, grenade.landing[2])) && (picked.length === 0 || picked.includes(grenade.thrower_steamid64)),
      ),
    [ofBuy, calibration, levels, picked],
  );
  const counts = useMemo(() => {
    const out = new Map<TeamGrenadeKind, number>();
    for (const grenade of onLevel) out.set(grenade.kind, (out.get(grenade.kind) ?? 0) + 1);
    return out;
  }, [onLevel]);
  const ofKinds = useMemo(() => onLevel.filter((grenade) => kinds.has(grenade.kind)), [onLevel, kinds]);
  // The tramo counts the throw by when the grenade went off, the moment the histogram bins too.
  const drawn = useMemo(() => ofKinds.filter((grenade) => inTramo(tramo, grenade.seconds, plantOf(grenade.match_id, grenade.round))), [ofKinds, tramo, plantOf]);
  const selected = useMemo(
    () => (box === null ? null : new Set(drawn.filter((grenade) => grenadeInBox({ grenade, box, area, floor: { calibration, level: boxLevel } })).map(grenadeId))),
    [drawn, box, area, calibration, boxLevel],
  );
  const matching = useMemo(() => (selected === null ? drawn : drawn.filter((grenade) => selected.has(grenadeId(grenade)))), [drawn, selected]);

  const spots = useMemo(
    (): LabelledSpot[] =>
      groupSpots(matching).map((spot) => ({
        spot,
        label: spotLabel(spot.kind, { landing: spot.landPlace === '' ? null : spot.landPlace, origin: spot.throwPlace === '' ? null : spot.throwPlace }, place),
      })),
    [matching, place],
  );
  const spotOf = useMemo(() => new Map(spots.flatMap(({ spot }) => spot.grenades.map((grenade) => [grenadeId(grenade), spot.key] as const))), [spots]);
  const openSpot = open === null ? undefined : spots.find(({ spot }) => spot.key === open);
  const shown = openSpot === undefined ? matching : openSpot.spot.grenades;
  const thrown = useMemo(() => thrownRounds(shown), [shown]);
  const matchingRounds = invert ? Math.max(0, total - thrown.size) : thrown.size;
  const rows = useMemo(() => roundRows(scopeRounds, shown, invert), [scopeRounds, shown, invert]);
  const followUp = useMemo(() => (openSpot === undefined ? null : followUpOf(openSpot.spot.grenades, scopeRounds)), [openSpot, scopeRounds]);
  const clock = useMemo(() => clockBins(ofKinds, plantOf), [ofKinds, plantOf]);
  const stack = picked.map((steamid64) => ({ steamid64, token: tokens.get(steamid64) ?? '--fg-3' }));
  const byPlayer = useCallback((grenade: TeamGrenade) => tokens.get(grenade.thrower_steamid64) ?? GRENADE_KIND_TOKEN[grenade.kind], [tokens]);
  const replayRefs = (invert ? rows.map(({ round }) => ({ matchId: round.match_id, round: round.round, tick: round.tick })) : spotReplayRefs(shown)).slice(0, REPLAY_MAX_REFS);
  const savingFollowUp = useMemo(
    () => (savingPlay === null ? null : followUpOf(savingPlay.flatMap(({ spot }) => spot.grenades), scopeRounds)),
    [savingPlay, scopeRounds],
  );
  const missingOrigins = matching.filter((grenade) => grenade.origin === null).length;
  const chosenSpots = spots.filter(({ spot }) => chosen.includes(spot.key));
  const counter = denominatorText({ count: matchingRounds, total, side, buy });

  const toggleKind = (kind: TeamGrenadeKind): void =>
    setKinds((current) => {
      const next = new Set(current);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });
  const choose = (key: string): void =>
    setChosen((current) => (current.includes(key) ? current.filter((entry) => entry !== key) : [...current, key].slice(0, PRACTICE_MAX_SPOTS)));

  let content: ReactNode;
  if (results === 'rounds' || invert) content = <GrenadeRoundList rows={rows} matchLabel={matchLabel} />;
  else if (openSpot !== undefined && followUp !== null) {
    content = (
      <GrenadeSpotPanel
        spot={openSpot}
        followUp={followUp}
        denominator={denominatorText({ count: followUp.rounds, total, side, buy })}
        place={place}
        matchLabel={matchLabel}
        busy={practice.state.kind === 'busy'}
        onPractice={practice.launch}
        onSaveLineup={(from, label) => setSaving({ from, label })}
        onSavePlay={teamId === null || side === '' ? null : () => setSavingPlay([openSpot])}
        onBack={() => setOpen(null)}
      />
    );
  } else if (spots.length === 0) {
    content = (
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
    );
  } else {
    content = (
      <GrenadeSpotList
        spots={spots}
        total={total}
        highlight={hover}
        onHover={setHover}
        chosen={new Set(chosen)}
        onChoose={choose}
        full={chosenSpots.length >= PRACTICE_MAX_SPOTS}
        busy={practice.state.kind === 'busy'}
        onPractice={practice.launch}
        onOpen={setOpen}
      />
    );
  }

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
          {rivals === undefined ? null : (
            <Toggle variant="filter" size="sm" pressed={rivals.on} onPressedChange={rivals.onChange}>
              {rivals.label}
            </Toggle>
          )}
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
        {pagePlayers === undefined ? <GrenadePlayerPicker players={throwers} picked={ownPlayers} onChange={setOwnPlayers} /> : null}
        <div className="flex flex-wrap items-center gap-2">
          <Segmented label="La caja acota" value={area} options={AREAS} onChange={setArea} />
          {box === null ? (
            <span className="text-body-sm text-fg-3">Arrastra sobre el radar para dibujar la caja; con zoom, Mayús + arrastrar.</span>
          ) : (
            <Button type="button" variant="ghost" size="xs" onClick={() => setBox(null)}>
              <X aria-hidden />
              Quitar caja
            </Button>
          )}
        </div>
        <GrenadeRadar
          geometry={geometry}
          levels={levels}
          grenades={drawn}
          spotOf={spotOf}
          selected={selected}
          highlight={open ?? hover}
          box={box === null ? null : { box, level: boxLevel }}
          onBox={(next) => {
            setBox(next?.box ?? null);
            if (next !== null) setBoxLevel(next.level);
          }}
          onHover={setHover}
          tokenOf={picked.length === 0 ? undefined : byPlayer}
          badge={
            openSpot === undefined
              ? null
              : {
                  at: openSpot.spot.landing,
                  level: levelForAltitude(calibration, openSpot.spot.grenades[0]?.landing[2] ?? 0),
                  text: `${openSpot.spot.rounds} ${openSpot.spot.rounds === 1 ? 'ronda' : 'rondas'}`,
                }
          }
        />
        <TramoBar tramo={tramo} onChange={setTramo} marks={<GrenadeClockMarks bins={clock.bins} stack={stack} />} />
        <GrenadeAfterPlant bin={clock.afterPlant} stack={stack} names={names} />
      </section>

      <section aria-labelledby="granadas-posiciones" className="flex flex-col gap-2 lg:sticky lg:top-[calc(var(--shell-strip-height)+3.5rem)] lg:max-h-[calc(100dvh-var(--shell-strip-height)-4.5rem)] lg:overflow-y-auto">
        <header className="flex flex-col gap-2">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 id="granadas-posiciones" className="text-body font-semibold text-fg-1 tabular-nums">
              {counter}
              <span className="ml-2 text-body-sm font-normal text-fg-2">{invert ? 'sin la granada' : `con ${matching.length} ${matching.length === 1 ? 'lanzamiento' : 'lanzamientos'}`}</span>
            </h2>
            {openSpot === undefined ? null : (
              <Button type="button" variant="ghost" size="xs" onClick={() => setOpen(null)}>
                <X aria-hidden />
                {openSpot.label}
              </Button>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Segmented label="Resultados" value={results} options={RESULTS} onChange={setResults} />
            <Toggle variant="filter" size="sm" pressed={invert} onPressedChange={setInvert} title="Las rondas del alcance en las que no se tiró">
              Invertir
            </Toggle>
            <Button asChild variant="outline" size="sm" className={replayRefs.length === 0 ? 'pointer-events-none opacity-50' : undefined}>
              <Link href={superposedReplayHref(replayRefs, openSpot?.label ?? counter)} aria-disabled={replayRefs.length === 0}>
                <Layers aria-hidden />
                Reproducir {replayRefs.length}/{total} rondas
              </Link>
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={chosenSpots.length === 0 || practice.state.kind === 'busy'}
              onClick={() => practice.launch(practiceSpotsForSpots(chosenSpots))}
            >
              <Crosshair aria-hidden />
              {chosenSpots.length === 0 ? 'Practicar selección' : `Practicar selección (${chosenSpots.length})`}
            </Button>
            {teamId === null || side === '' ? null : (
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={chosenSpots.length === 0}
                title="Guarda las granadas elegidas como una jugada de la Libreta"
                onClick={() => setSavingPlay(chosenSpots)}
              >
                <BookPlus aria-hidden />
                {chosenSpots.length === 0 ? 'Crear táctica' : `Crear táctica (${chosenSpots.length})`}
              </Button>
            )}
          </div>
        </header>
        <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
        {chosenSpots.length >= PRACTICE_MAX_SPOTS ? <p className="text-body-sm text-fg-2">Caben {PRACTICE_MAX_SPOTS} posiciones por práctica.</p> : null}
        {missingOrigins > 0 ? (
          <p className="text-body-sm text-fg-2">
            {missingOrigins} {missingOrigins === 1 ? 'granada no tiene' : 'granadas no tienen'} posición de lanzamiento: son de demos analizadas con una versión
            anterior. Studio las vuelve a analizar al arrancar.
          </p>
        ) : null}
        {content}
      </section>
      {saving ? <SaveLineupDialog from={saving.from} label={saving.label} onClose={() => setSaving(null)} /> : null}
      {savingPlay !== null && savingPlay.length > 0 && savingFollowUp !== null && teamId !== null && side !== '' ? (
        <SaveGrenadePlayDialog
          defaultTitle={savingPlay.map(({ label }) => label).join(' + ')}
          play={(title) =>
            playFromGrenadeSpots({ teamId, map, side, title, spots: savingPlay.map(({ spot }) => spot), rounds: scopeRounds, followUp: savingFollowUp, place })
          }
          onClose={() => setSavingPlay(null)}
        />
      ) : null}
    </div>
  );
}
