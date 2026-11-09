'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMemo, useState, type ReactNode } from 'react';
import { ChevronLeft, ChevronRight, Crosshair, MapPinned, NotebookPen, Play } from 'lucide-react';
import {
  lineupHref,
  pct,
  practiceSpot,
  PRO_KIND_LABEL,
  roundClock,
  saveProLineup,
  shownClip,
  techniqueLabel,
  type ProLineup,
  type ProLineupMap,
  type ProUsage,
} from '@/lib/api/pro-lineups';
import { isLibraryOffline } from '@/lib/api/library';
import { prettyMapName, formatShortDate } from '@/lib/format';
import { landingSpots, plainTitle, spotHref, spotLabel, spotOf, type ProSpot } from '@/lib/pro-lineups-view';
import { levelForAltitude } from '@/lib/tactical-transform';
import { PracticeResult, usePracticeLaunch } from '@/components/patterns/practice-launch';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { ProLoadProblem } from './lineup-maps';
import { ProLineupRadar, type RadarThrow } from './pro-lineup-radar';
import { SetposCopy } from './setpos-copy';
import { ThrowNumber } from './spot-panel';
import { ThrowClip } from './throw-clip';
import { useProLineupMap } from './use-pro-lineups';

type Who = 'teams' | 'players' | 'events';
const WHO: readonly { value: Who; label: string }[] = [
  { value: 'teams', label: 'Equipos' },
  { value: 'players', label: 'Jugadores' },
  { value: 'events', label: 'Torneos' },
];

type SaveState = { kind: 'idle' } | { kind: 'busy' } | { kind: 'saved'; id: string } | { kind: 'failed'; message: string };

function DetailSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando lineup" className="flex flex-col gap-4">
      <Skeleton className="h-5 w-40" />
      <div className="flex flex-col gap-2">
        <Skeleton className="h-8 w-2/3" />
        <Skeleton className="h-4 w-1/3" />
      </div>
      <div className="grid gap-5 lg:grid-cols-[minmax(0,1.4fr)_minmax(20rem,1fr)]">
        <Skeleton className="aspect-video w-full" />
        <div className="flex flex-col gap-3">
          <Skeleton className="h-36 w-full" />
          <Skeleton className="aspect-square w-full max-w-80" />
        </div>
      </div>
    </div>
  );
}

/** One pro lineup: how to throw it, how pros use it and where to see it. */
export function LineupDetail({ map, id }: { map: string; id: string }): ReactNode {
  const [attempt, setAttempt] = useState(0);
  const load = useProLineupMap(map, attempt);
  if (load.kind === 'loading') return <DetailSkeleton />;
  if (load.kind !== 'ready') return <ProLoadProblem load={load} onRetry={() => setAttempt((n) => n + 1)} map={map} />;
  const lineup = load.data.lineups.find((l) => l.id === id);
  if (lineup === undefined) {
    return (
      <EmptyState
        icon={MapPinned}
        title="Este lineup ya no está en la biblioteca"
        description="Los lineups se recalculan cada día con las partidas pro nuevas, y este dejó de cumplir el mínimo de usos."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={`/lineups/${map}`}>Ver lineups de {prettyMapName(map)}</Link>
          </Button>
        }
      />
    );
  }
  return <Detail data={load.data} lineup={lineup} />;
}

function Detail({ data, lineup }: { data: ProLineupMap; lineup: ProLineup }): ReactNode {
  const router = useRouter();
  const practice = usePracticeLaunch(data.map);
  const [save, setSave] = useState<SaveState>({ kind: 'idle' });
  const [chosenClip, setChosenClip] = useState<string | null>(null);
  const spot = useMemo(() => spotOf(landingSpots(data.lineups), lineup.id), [data.lineups, lineup.id]);
  const siblings = spot === undefined ? [] : spot.lineups.filter((l) => l.id !== lineup.id);
  const geometry = data.geometry;
  const hasClip = shownClip(lineup, chosenClip) !== null;

  const onSave = (): void => {
    setSave({ kind: 'busy' });
    saveProLineup(data.map, lineup.id)
      .then((saved) => setSave({ kind: 'saved', id: saved.id }))
      .catch((error: unknown) =>
        setSave({ kind: 'failed', message: isLibraryOffline(error) ? 'El servicio local no responde.' : 'No se pudo guardar en la Libreta.' }),
      );
  };

  const radar =
    geometry === undefined ? (
      <p className="rounded-lg border border-border bg-surface-1 px-3 py-3 text-body-sm text-fg-2">Este mapa aún no trae radar.</p>
    ) : (
      <figure className="flex flex-col gap-1.5">
        <ProLineupRadar
          geometry={geometry}
          level={levelForAltitude(geometry.calibration, lineup.landing[2])}
          throws={[...siblings.map((l) => radarThrow(l, 'faint', spot)), radarThrow(lineup, 'strong', spot)]}
          onThrow={(id) => {
            if (id !== lineup.id) router.push(lineupHref(data.map, { id }));
          }}
          label={`Radar: se lanza desde ${lineup.throw_place} y cae en ${lineup.land_place}`}
        />
        <figcaption className="text-meta text-fg-3">
          {siblings.length === 0 ? 'Desde dónde se lanza y dónde cae.' : 'Los puntos tenues son otras formas de tirarla al mismo sitio; pulsa una para abrirla.'}
        </figcaption>
      </figure>
    );

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Link href={spotHref(data.map, lineup)} className="inline-flex items-center gap-1 text-body-sm text-fg-2 hover:text-fg-1">
            <ChevronLeft aria-hidden className="size-4" />
            {spot === undefined ? `Lineups de ${prettyMapName(data.map)}` : `${spotLabel(spot)} en ${prettyMapName(data.map)}`}
          </Link>
          {spot !== undefined && spot.lineups.length > 1 ? <SpotSteps map={data.map} spot={spot} lineup={lineup} /> : null}
        </div>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <h1 className="text-section font-semibold text-fg-1">{plainTitle(lineup.title)}</h1>
            <p className="mt-1.5 text-body-sm text-fg-2">
              {PRO_KIND_LABEL[lineup.kind]} de {lineup.side}, {techniqueLabel(lineup).toLowerCase()}
              {lineup.insta ? ', se lanza nada más empezar la ronda' : ''}.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant="outline" size="sm" disabled={save.kind === 'busy' || save.kind === 'saved'} onClick={onSave}>
              <NotebookPen aria-hidden />
              {save.kind === 'saved' ? 'Guardado' : 'Guardar en la Libreta'}
            </Button>
            <Button type="button" size="sm" disabled={practice.state.kind === 'busy'} onClick={() => practice.launch([practiceSpot(lineup)])}>
              <Crosshair aria-hidden />
              Practicar en CS2
            </Button>
          </div>
        </div>
        <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
        {save.kind === 'saved' ? (
          <p role="status" className="text-body-sm text-fg-2">
            Guardado en la Libreta.{' '}
            <Link href={`/libreta/lineups/${save.id}`} className="text-fg-1 underline underline-offset-2">
              Abrirlo
            </Link>
          </p>
        ) : null}
        {save.kind === 'failed' ? (
          <p role="alert" className="text-body-sm text-destructive">
            {save.message}
          </p>
        ) : null}
      </div>

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1.4fr)_minmax(20rem,1fr)]">
        <section aria-label={hasClip ? 'Vídeo del lanzamiento' : 'Radar'}>{hasClip ? <ThrowClip lineup={lineup} chosen={chosenClip} /> : radar}</section>
        <div className="flex flex-col gap-5">
          <HowToThrow lineup={lineup} hasClip={hasClip} />
          {hasClip ? <div className="max-w-96">{radar}</div> : null}
        </div>
      </div>

      <div className="grid items-start gap-5 lg:grid-cols-2">
        <ProUsage lineup={lineup} data={data} />
        <WhoThrows lineup={lineup} />
      </div>
      <Context lineup={lineup} map={data.map} siblings={siblings} spot={spot} />
      <Examples lineup={lineup} shown={shownClip(lineup, chosenClip)?.clip ?? null} onShow={setChosenClip} />
    </div>
  );
}

function radarThrow(lineup: ProLineup, state: RadarThrow['state'], spot: ProSpot | undefined): RadarThrow {
  const index = spot === undefined ? -1 : spot.lineups.indexOf(lineup);
  return {
    id: lineup.id,
    kind: lineup.kind,
    side: lineup.side,
    from: [lineup.origin[0], lineup.origin[1]],
    to: [lineup.landing[0], lineup.landing[1]],
    label: `Desde ${lineup.throw_place || 'zona sin nombre'}, ${techniqueLabel(lineup).toLowerCase()}`,
    number: index < 0 ? undefined : index + 1,
    state,
  };
}

/** Previous and next way to throw to the same spot, most thrown first, as in the explorer. */
function SpotSteps({ map, spot, lineup }: { map: string; spot: ProSpot; lineup: ProLineup }): ReactNode {
  const index = spot.lineups.findIndex((l) => l.id === lineup.id);
  const previous = spot.lineups[index - 1];
  const next = spot.lineups[index + 1];
  return (
    <nav aria-label="Otras formas de tirarla al mismo sitio" className="flex items-center gap-1">
      {previous === undefined ? (
        <Button type="button" variant="ghost" size="icon-xs" aria-label="Lineup anterior" disabled>
          <ChevronLeft aria-hidden />
        </Button>
      ) : (
        <Button asChild variant="ghost" size="icon-xs" aria-label="Lineup anterior">
          <Link href={lineupHref(map, previous)}>
            <ChevronLeft aria-hidden />
          </Link>
        </Button>
      )}
      <span className="text-meta text-fg-2 tabular-nums">
        {index + 1} de {spot.lineups.length} a este sitio
      </span>
      {next === undefined ? (
        <Button type="button" variant="ghost" size="icon-xs" aria-label="Lineup siguiente" disabled>
          <ChevronRight aria-hidden />
        </Button>
      ) : (
        <Button asChild variant="ghost" size="icon-xs" aria-label="Lineup siguiente">
          <Link href={lineupHref(map, next)}>
            <ChevronRight aria-hidden />
          </Link>
        </Button>
      )}
    </nav>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }): ReactNode {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-meta text-fg-2">{label}</dt>
      <dd className="text-body-sm font-medium text-fg-1 tabular-nums">{children}</dd>
    </div>
  );
}

function HowToThrow({ lineup, hasClip }: { lineup: ProLineup; hasClip: boolean }): ReactNode {
  const accuracy = lineup.attempts === 0 ? null : lineup.landed / lineup.attempts;
  return (
    <section aria-labelledby="como-lanzarlo" className="flex flex-col gap-3 rounded-lg border border-border bg-surface-1 p-4">
      <h2 id="como-lanzarlo" className="text-body font-semibold text-fg-1">
        Cómo lanzarlo
      </h2>
      <dl className="grid grid-cols-2 gap-3">
        <Fact label="Desde">{lineup.throw_place || 'Zona sin nombre'}</Fact>
        <Fact label="Técnica">{techniqueLabel(lineup)}</Fact>
        <Fact label="Tiempo en el aire">{lineup.air_time.toFixed(1)} s</Fact>
        <Fact label="Cae donde debe">{accuracy === null ? 'Sin datos' : `${pct(accuracy)} (${lineup.landed} de ${lineup.attempts})`}</Fact>
      </dl>
      <SetposCopy lineup={lineup} />
      {hasClip ? null : (
        <p className="text-meta text-fg-2">
          Aún no hay vídeo de este lineup: se graba cuando entra una partida pro nueva con este lanzamiento. Con Practicar en CS2 apareces en la posición y la mira.
        </p>
      )}
    </section>
  );
}

function ClockBars({ buckets }: { buckets: readonly number[] }): ReactNode {
  const max = Math.max(1, ...buckets);
  const peak = buckets.indexOf(max);
  const summary = `Sobre todo entre ${roundClock(peak * 10)} y ${roundClock(peak * 10 + 10)}`;
  return (
    <figure className="flex flex-col gap-1.5">
      <figcaption className="flex items-baseline justify-between gap-2 text-body-sm">
        <span className="font-medium text-fg-1">Cuándo lo tiran</span>
        <span className="text-meta text-fg-2">{summary}</span>
      </figcaption>
      <div role="img" aria-label={summary} className="flex h-16 items-end gap-px border-b border-border">
        {buckets.map((count, index) => (
          <div
            key={index}
            title={`${roundClock(index * 10)} a ${roundClock(index * 10 + 10)}: ${count}`}
            className="flex-1 rounded-t-xs bg-fg-3"
            style={{ height: `${(count / max) * 100}%` }}
          />
        ))}
      </div>
      <div aria-hidden className="flex justify-between text-meta text-fg-3 tabular-nums">
        <span>1:55</span>
        <span>1:15</span>
        <span>0:35</span>
        <span>después</span>
      </div>
    </figure>
  );
}

function trendWord(now: number, before: number): string {
  if (now > before) return 'sube';
  if (now < before) return 'baja';
  return 'igual';
}

function ProUsage({ lineup, data }: { lineup: ProLineup; data: ProLineupMap }): ReactNode {
  return (
    <section aria-labelledby="uso-pro" className="flex flex-col gap-3">
      <h2 id="uso-pro" className="text-body font-semibold text-fg-1">
        Uso en partidas pro
      </h2>
      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <Fact label={`Lanzamientos (${data.recent_days} días)`}>
          {lineup.throws.recent}
          {lineup.throws.all === lineup.throws.recent ? null : <span className="font-normal text-fg-2"> de {lineup.throws.all}</span>}
        </Fact>
        <Fact label={`Rondas ${lineup.side} con este lineup`}>{pct(lineup.round_share)}</Fact>
        <Fact label="Rondas ganadas">{pct(lineup.round_win)}</Fact>
        <Fact label="Mapas">{lineup.matches.recent}</Fact>
        <Fact label="Equipos">{lineup.team_count}</Fact>
        {lineup.trend === undefined ? null : (
          <Fact label="Por 100 rondas">
            {lineup.trend[0].toFixed(1)}{' '}
            <span className="font-normal text-fg-2">
              {trendWord(lineup.trend[0], lineup.trend[1])} desde {lineup.trend[1].toFixed(1)}
            </span>
          </Fact>
        )}
      </dl>
      <ClockBars buckets={lineup.clock.buckets} />
      <p className="text-meta text-fg-3">Visto por última vez el {formatShortDate(lineup.last_seen)}.</p>
    </section>
  );
}

function UsageRows({ rows, detail }: { rows: readonly ProUsage[]; detail?: (row: ProUsage) => string }): ReactNode {
  if (rows.length === 0) return <p className="text-body-sm text-fg-2">Sin datos en este periodo.</p>;
  return (
    <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
      {rows.map((row) => (
        <li key={row.key} className="flex items-baseline gap-3 px-3 py-2 text-body-sm">
          <span className="min-w-0 truncate text-fg-1">{row.name}</span>
          <span className="ml-auto shrink-0 text-fg-2 tabular-nums">
            {row.count} {detail === undefined ? '' : detail(row)}
          </span>
        </li>
      ))}
    </ul>
  );
}

function WhoThrows({ lineup }: { lineup: ProLineup }): ReactNode {
  const [who, setWho] = useState<Who>('teams');
  const rows = { teams: lineup.teams, players: lineup.players, events: lineup.events }[who];
  return (
    <section aria-labelledby="quien-lo-tira" className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="quien-lo-tira" className="text-body font-semibold text-fg-1">
          Quién lo tira
        </h2>
        <Segmented label="Agrupar por" value={who} options={WHO} onChange={setWho} />
      </div>
      <UsageRows
        rows={rows}
        detail={who === 'teams' ? (row) => (row.detail === undefined ? 'rondas' : `rondas, ${row.detail} % de sus rondas ${lineup.side}`) : () => 'lanzamientos'}
      />
    </section>
  );
}

function Context({ lineup, map, siblings, spot }: { lineup: ProLineup; map: string; siblings: readonly ProLineup[]; spot: ProSpot | undefined }): ReactNode {
  const sites = lineup.plant_sites.filter((site) => site.key !== '');
  return (
    <div className="grid items-start gap-5 lg:grid-cols-2">
      <section aria-labelledby="con-que-va" className="flex flex-col gap-2">
        <h2 id="con-que-va" className="text-body font-semibold text-fg-1">
          Se tira con
        </h2>
        {lineup.with.length === 0 ? (
          <p className="text-body-sm text-fg-2">Suele ir solo: ningún otro lineup sale en los 15 segundos de alrededor.</p>
        ) : (
          <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
            {lineup.with.map((row) => (
              <li key={row.key}>
                <Link href={lineupHref(map, { id: row.key })} className="flex items-baseline gap-3 px-3 py-2 text-body-sm hover:bg-surface-2">
                  <span className="min-w-0 truncate text-fg-1">{plainTitle(row.name)}</span>
                  <span className="ml-auto shrink-0 text-fg-2 tabular-nums">{pct(row.share)} de las rondas</span>
                </Link>
              </li>
            ))}
          </ul>
        )}
        {lineup.side === 'T' && sites.length > 0 ? (
          <p className="text-body-sm text-fg-2">
            Cuando hay plant, suele ir a{' '}
            <span className="font-medium text-fg-1">
              {sites[0].key} ({pct(sites[0].share)})
            </span>
            .
          </p>
        ) : null}
      </section>
      {spot !== undefined && siblings.length > 0 ? (
        <section aria-labelledby="mismo-sitio" className="flex flex-col gap-2">
          <h2 id="mismo-sitio" className="text-body font-semibold text-fg-1">
            Otras formas de tirarla a {spot.place || 'esta zona'}
          </h2>
          <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
            {siblings.slice(0, 8).map((other) => (
              <li key={other.id}>
                <Link href={lineupHref(map, other)} className="flex items-center gap-3 px-3 py-2 text-body-sm hover:bg-surface-2">
                  <ThrowNumber side={other.side} number={spot.lineups.indexOf(other) + 1} />
                  <span className="min-w-0 truncate text-fg-1">
                    Desde {other.throw_place || 'zona sin nombre'}
                    <span className="text-fg-2"> · {techniqueLabel(other)}</span>
                  </span>
                  {other.clip ? <Play aria-label="Con vídeo" className="size-3.5 shrink-0 text-fg-2" /> : null}
                  <span className="ml-auto shrink-0 text-fg-2 tabular-nums">
                    {other.side} · {other.throws.recent} lanzamientos
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}

function Examples({ lineup, shown, onShow }: { lineup: ProLineup; shown: string | null; onShow: (clip: string) => void }): ReactNode {
  return (
    <section aria-labelledby="en-partidas" className="flex flex-col gap-2">
      <h2 id="en-partidas" className="text-body font-semibold text-fg-1">
        Últimas veces en partidas pro
      </h2>
      <div className="overflow-x-auto rounded-lg border border-border bg-surface-1">
        <table className="w-full text-left text-body-sm">
          <thead className="text-meta text-fg-2">
            <tr className="border-b border-border">
              <th className="px-3 py-2 font-medium">Jugador</th>
              <th className="px-3 py-2 font-medium">Equipo</th>
              <th className="px-3 py-2 font-medium">Torneo</th>
              <th className="px-3 py-2 font-medium">Ronda</th>
              <th className="px-3 py-2 font-medium">Fecha</th>
              <th className="px-3 py-2 font-medium">Vídeo</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {lineup.examples.map((example) => (
              <tr key={`${example.map_key}-${example.round}-${example.tick}`}>
                <td className="px-3 py-2 text-fg-1">{example.player}</td>
                <td className="px-3 py-2 text-fg-2">{example.team}</td>
                <td className="px-3 py-2 text-fg-2">{example.event}</td>
                <td className="px-3 py-2 text-fg-2 tabular-nums">
                  R{example.round} · {roundClock(example.clock)}
                </td>
                <td className="px-3 py-2 text-fg-2 tabular-nums">{formatShortDate(example.played_at)}</td>
                <td className="px-3 py-2">
                  {example.clip === undefined ? (
                    <span className="text-fg-3">Sin grabar</span>
                  ) : (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      aria-pressed={shown === example.clip}
                      disabled={shown === example.clip}
                      onClick={() => {
                        if (example.clip !== undefined) onShow(example.clip);
                        window.scrollTo({ top: 0, behavior: 'smooth' });
                      }}
                    >
                      <Play aria-hidden />
                      {shown === example.clip ? 'En pantalla' : 'Ver'}
                    </Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
