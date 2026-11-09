'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { getTeamBombsites } from '@/lib/api/library';
import type { ScoutEvidence } from '@/lib/api/scout';
import type { TacticalRate } from '@/lib/api/tactical';
import type { BombsiteMap, TeamBombsites as TeamBombsitesReport } from '@/lib/api/team-insights';
import { weightedMean } from '@/lib/analysis/compare';
import { RatePool, bombsiteKey, libraryMatchesText, poolBombsites, type AttackField, type DefenseField } from '@/lib/analysis/library-pool';
import { replayHref } from '@/lib/analysis/replay-link';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { prettyMapName } from '@/lib/format';
import { MapThumb } from '@/components/analysis/map-thumb';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';
import { HeatTable, type HeatCell, type HeatColumn, type HeatRow } from '@/components/ui/heat-table';
import { Skeleton } from '@/components/ui/skeleton';
import { useLibraryNorms } from '@/hooks/use-library-norms';

/** What the ▲▼ compare with, by the state of the library norms. */
function referenceText(norms: ReturnType<typeof useLibraryNorms>): string {
  if (norms === null) return 'Leyendo la referencia de la biblioteca.';
  if (norms === 'failed') return 'No se pudo leer la referencia de la biblioteca, así que no hay ▲▼.';
  return `La referencia suma los dos equipos de ${libraryMatchesText(norms.matches)} en todas sus fechas, este incluido; cuenta desde ${MIN_LIBRARY_ROUNDS} rondas.`;
}

/** Points of rate that earn the strongest tint. */
const SPREAD = 30;

type Rates = readonly TacticalRate[];

/** The team's own mean of a rate over every map and site in the report, the tint's baseline. */
function heat(rates: Rates, higherIsBetter = true): HeatColumn['heat'] {
  const baseline = weightedMean(rates, (rate) => rate.pct, (rate) => rate.total);
  return baseline === null ? undefined : { baseline, spread: SPREAD, higherIsBetter };
}

function columns(report: TeamBombsitesReport): { ct: HeatColumn[]; t: HeatColumn[] } {
  const sites = report.maps.flatMap((entry) => entry.sites);
  return {
    ct: [
      { id: 'site', header: 'Sitio' },
      { id: 'hit', header: 'Les atacan', align: 'end' },
      { id: 'held', header: 'Aguantan', align: 'end', heat: heat(sites.map((s) => s.ct.held)) },
      { id: 'plants', header: 'Plantan', align: 'end', heat: heat(sites.map((s) => s.ct.plants_allowed), false) },
      { id: 'won', header: 'Ganan', align: 'end', heat: heat(sites.map((s) => s.ct.won)) },
      { id: 'retakes', header: 'Retakes ganados', align: 'end', heat: heat(sites.map((s) => s.ct.retakes_won)) },
      { id: 'rounds', header: 'Rondas' },
    ],
    t: [
      { id: 'site', header: 'Sitio' },
      { id: 'attacks', header: 'Atacan', align: 'end' },
      { id: 'plants', header: 'Plantan', align: 'end', heat: heat(sites.map((s) => s.t.plants)) },
      { id: 'won', header: 'Ganan', align: 'end', heat: heat(sites.map((s) => s.t.won)) },
      { id: 'post', header: 'Ganan tras plantar', align: 'end', heat: heat(sites.map((s) => s.t.post_plant_won)) },
      { id: 'rounds', header: 'Rondas' },
    ],
  };
}

/** Under this many library rounds a site's pooled rate is too thin to compare with. */
const MIN_LIBRARY_ROUNDS = 8;

function rateCell(rate: TacticalRate, reference?: number | null): HeatCell {
  if (rate.total === 0) return { text: '', emptyLabel: 'Sin rondas' };
  return {
    text: `${Math.round(rate.pct)} %`,
    value: rate.pct,
    n: rate.total,
    reference,
    referenceText: reference === null || reference === undefined ? undefined : `Biblioteca ${Math.round(reference)} %`,
  };
}

function roundsCell(examples: readonly ScoutEvidence[], title: string): HeatCell {
  if (examples.length === 0) return { text: '' };
  const refs = examples.map((example) => ({ matchId: example.match_id, round: example.round, tick: example.tick }));
  return { text: examples.length === 1 ? 'Ver 1 ronda' : `Ver ${examples.length} rondas`, href: replayHref(refs, title) };
}

function MapTables({
  entry,
  report,
  heads,
  library,
}: {
  entry: BombsiteMap;
  report: TeamBombsitesReport;
  heads: { ct: HeatColumn[]; t: HeatColumn[] };
  library: RatePool | null;
}): ReactNode {
  const ref = (site: string, field: `ct.${DefenseField}` | `t.${AttackField}`): number | null | undefined =>
    library ? library.pct(bombsiteKey(entry.map, site, field), MIN_LIBRARY_ROUNDS) : undefined;
  const reference = library ? 'la biblioteca en el mismo sitio' : 'la media del equipo';
  const team = report.team_name;
  const minSample = report.min_reliable_sample;
  const name = prettyMapName(entry.map);
  const ct: HeatRow[] = entry.sites.map((site) => ({
    id: site.site,
    cells: {
      site: { text: site.site, tone: 'ct' },
      hit: rateCell(site.ct.hit, ref(site.site, 'ct.hit')),
      held: rateCell(site.ct.held, ref(site.site, 'ct.held')),
      plants: rateCell(site.ct.plants_allowed, ref(site.site, 'ct.plants_allowed')),
      won: rateCell(site.ct.won, ref(site.site, 'ct.won')),
      retakes: rateCell(site.ct.retakes_won, ref(site.site, 'ct.retakes_won')),
      rounds: roundsCell(site.ct.examples, `${team} defendiendo ${site.site} en ${name}`),
    },
  }));
  const t: HeatRow[] = entry.sites.map((site) => ({
    id: site.site,
    cells: {
      site: { text: site.site, tone: 't' },
      attacks: rateCell(site.t.attacks, ref(site.site, 't.attacks')),
      plants: rateCell(site.t.plants, ref(site.site, 't.plants')),
      won: rateCell(site.t.won, ref(site.site, 't.won')),
      post: rateCell(site.t.post_plant_won, ref(site.site, 't.post_plant_won')),
      rounds: roundsCell(site.t.examples, `${team} atacando ${site.site} en ${name}`),
    },
  }));
  return (
    <section aria-label={name} className="flex flex-col gap-2">
      <h2 className="flex items-center gap-2 text-body font-semibold text-fg-1">
        <MapThumb map={entry.map} className="h-6 w-10" crop={1.4} />
        {name}
        <span className="text-body-sm font-normal text-fg-2">
          {entry.matches === 1 ? '1 partida' : `${entry.matches} partidas`}, {entry.ct_rounds} rondas de CT y {entry.t_rounds} de T
        </span>
      </h2>
      <div className="grid gap-2 @[96rem]/content:grid-cols-2">
        <HeatTable label={`Defendiendo en ${name}`} columns={heads.ct} rows={ct} minSample={minSample} sampleNoun="rondas" empty="Sin rondas de CT." reference={reference} legend={false} />
        <HeatTable label={`Atacando en ${name}`} columns={heads.t} rows={t} minSample={minSample} sampleNoun="rondas" empty="Sin rondas de T." reference={reference} />
      </div>
    </section>
  );
}

/** Per map and site, how the team defends and attacks it, with the rounds behind each number. */
export function TeamBombsites({
  teamId,
  scope,
  lineup,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [data, setData] = useState<TeamBombsitesReport | null | 'failed'>(null);
  const [now] = useState(() => Date.now());
  const { map, opponent } = scope;
  const { from, to } = scopeDateRange(scope.period, now);
  const norms = useLibraryNorms(map);
  const library = norms === null || norms === 'failed' ? null : poolBombsites(norms.bombsites);

  useEffect(() => {
    let active = true;
    setData(null);
    onCounts(null);
    getTeamBombsites(teamId, { maps: map === '' ? [] : [map], opponent_ids: opponent === '' ? [] : [opponent], from, to, lineup })
      .then((report) => {
        if (!active) return;
        setData(report);
        onCounts({
          maps: report.match_count,
          rounds: report.maps.reduce((sum, entry) => sum + entry.ct_rounds + entry.t_rounds, 0),
        });
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, map, opponent, from, to, lineup, onCounts]);

  if (data === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudieron leer los sitios de este equipo. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Leyendo los sitios" className="grid gap-2 @[96rem]/content:grid-cols-2">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }
  if (data.maps.length === 0) {
    return <p className="text-body-sm text-fg-2">Sin partidas analizadas con este filtro. Amplía las fechas o quita el mapa o el rival.</p>;
  }
  const heads = columns(data);
  return (
    <div className="flex flex-col gap-4">
      {data.maps.map((entry) => (
        <MapTables key={entry.map} entry={entry} report={data} heads={heads} library={library} />
      ))}
      {norms === null || norms === 'failed' ? <p className="text-body-sm text-fg-2">{referenceText(norms)}</p> : null}
      {data.skipped.length > 0 ? (
        <p className="text-body-sm text-fg-2">
          {data.skipped.length === 1 ? 'Una partida sin análisis no cuenta.' : `${data.skipped.length} partidas sin análisis no cuentan.`}
        </p>
      ) : null}
      <details className="measure-read text-meta text-fg-3">
        <DisclosureSummary className="min-h-10 text-body-sm text-fg-2">Cómo se cuenta</DisclosureSummary>
        <p className="pb-2">
          Defendiendo, «Les atacan» cuenta sobre sus rondas de CT del mapa; aguantar, plantar y ganar, sobre las rondas en que les atacan ese
          sitio; los retakes, sobre las plantas. Atacando, «Atacan» cuenta sobre sus rondas de T y «Ganan tras plantar», sobre sus
          plantas. {norms === null || norms === 'failed' ? null : referenceText(norms)}
        </p>
      </details>
    </div>
  );
}
