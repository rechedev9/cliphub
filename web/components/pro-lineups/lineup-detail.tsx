'use client';

import Link from 'next/link';
import { useMemo, useState, type ReactNode } from 'react';
import { ArrowLeft, Copy, Crosshair, MapPinned, NotebookPen } from 'lucide-react';
import {
  lineupHref,
  pct,
  practiceSpot,
  PRO_KIND_LABEL,
  roundClock,
  sameLanding,
  saveProLineup,
  setposCommand,
  techniqueLabel,
  type ProLineup,
  type ProLineupMap,
  type ProUsage,
} from '@/lib/api/pro-lineups';
import { isLibraryOffline } from '@/lib/api/library';
import { prettyMapName, formatShortDate } from '@/lib/format';
import { levelForAltitude } from '@/lib/tactical-transform';
import { PracticeResult, usePracticeLaunch } from '@/components/patterns/practice-launch';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { ProLoadProblem } from './lineup-maps';
import { ProLineupRadar } from './pro-lineup-radar';
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
      <Skeleton className="h-8 w-2/3" />
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(20rem,1fr)]">
        <Skeleton className="aspect-square w-full" />
        <div className="flex flex-col gap-2">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      </div>
    </div>
  );
}

/** One pro lineup: where to stand and aim, how pros use it and where to see it. */
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
  const practice = usePracticeLaunch(data.map);
  const [save, setSave] = useState<SaveState>({ kind: 'idle' });
  const siblings = useMemo(() => sameLanding(data.lineups, lineup), [data.lineups, lineup]);
  const geometry = data.geometry;

  const onSave = (): void => {
    setSave({ kind: 'busy' });
    saveProLineup(data.map, lineup.id)
      .then((saved) => setSave({ kind: 'saved', id: saved.id }))
      .catch((error: unknown) =>
        setSave({ kind: 'failed', message: isLibraryOffline(error) ? 'El servicio local no responde.' : 'No se pudo guardar en la Libreta.' }),
      );
  };

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3">
        <Link href={`/lineups/${data.map}`} className="inline-flex items-center gap-1 self-start text-body-sm text-fg-2 hover:text-fg-1">
          <ArrowLeft aria-hidden className="size-4" />
          Lineups de {prettyMapName(data.map)}
        </Link>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <h1 className="text-section font-semibold text-fg-1">{lineup.title}</h1>
            <p className="mt-1.5 text-body-sm text-fg-2">
              {PRO_KIND_LABEL[lineup.kind]} de {lineup.side}
              {lineup.insta ? ', se lanza nada más empezar la ronda' : ''}. Número {lineup.rank.position} de {lineup.rank.of} que caen en{' '}
              {lineup.land_place || 'esta zona'}.
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

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1.2fr)_minmax(20rem,1fr)]">
        <section aria-label="Radar" className="flex flex-col gap-2">
          {geometry === undefined ? (
            <p className="text-body-sm text-fg-2">Este mapa aún no trae radar.</p>
          ) : (
            <ProLineupRadar
              geometry={geometry}
              level={levelForAltitude(geometry.calibration, lineup.origin[2])}
              lineups={[lineup, ...siblings]}
              focus={lineup}
              label={`Radar: se lanza desde ${lineup.throw_place} y cae en ${lineup.land_place}`}
            />
          )}
          <p className="text-meta text-fg-3">Punto: desde dónde se lanza. Círculo: dónde cae. Los puntos tenues son otros lineups que caen ahí.</p>
        </section>

        <div className="flex flex-col gap-5">
          <HowToThrow lineup={lineup} />
          <ProUsage lineup={lineup} data={data} />
        </div>
      </div>

      <div className="grid items-start gap-5 lg:grid-cols-2">
        <WhoThrows lineup={lineup} />
        <Context lineup={lineup} map={data.map} siblings={siblings} />
      </div>
      <Examples lineup={lineup} />
    </div>
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

function HowToThrow({ lineup }: { lineup: ProLineup }): ReactNode {
  const [copied, setCopied] = useState(false);
  const command = setposCommand(lineup);
  const accuracy = lineup.attempts === 0 ? null : lineup.landed / lineup.attempts;
  return (
    <section aria-labelledby="como-lanzarlo" className="flex flex-col gap-3">
      <h2 id="como-lanzarlo" className="text-body font-semibold text-fg-1">
        Cómo lanzarlo
      </h2>
      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <Fact label="Técnica">{techniqueLabel(lineup)}</Fact>
        <Fact label="Desde">{lineup.throw_place || 'Zona sin nombre'}</Fact>
        <Fact label="Tiempo en el aire">{lineup.air_time.toFixed(1)} s</Fact>
        <Fact label="Cae donde debe">
          {accuracy === null ? 'Sin datos' : `${pct(accuracy)} (${lineup.landed} de ${lineup.attempts})`}
        </Fact>
      </dl>
      <div className="flex flex-col gap-1.5">
        <span className="text-meta text-fg-2">Posición y mira para la consola</span>
        <div className="flex flex-wrap items-center gap-2">
          <code className="rounded-sm bg-surface-3 px-2 py-1 font-mono text-body-sm break-all text-fg-1">{command}</code>
          <Button
            type="button"
            variant="ghost"
            size="xs"
            onClick={() => {
              void navigator.clipboard
                .writeText(command)
                .then(() => setCopied(true))
                .catch(() => setCopied(false));
            }}
          >
            <Copy aria-hidden />
            {copied ? 'Copiado' : 'Copiar'}
          </Button>
        </div>
      </div>
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
  const [now, before] = lineup.trend;
  return (
    <section aria-labelledby="uso-pro" className="flex flex-col gap-3">
      <h2 id="uso-pro" className="text-body font-semibold text-fg-1">
        Uso en partidas pro
      </h2>
      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <Fact label={`Lanzamientos (${data.recent_days} días)`}>
          {lineup.throws.recent} <span className="font-normal text-fg-2">de {lineup.throws.all}</span>
        </Fact>
        <Fact label={`Rondas ${lineup.side} con este lineup`}>{pct(lineup.round_share)}</Fact>
        <Fact label="Rondas ganadas">{pct(lineup.round_win)}</Fact>
        <Fact label="Mapas">{lineup.matches.recent}</Fact>
        <Fact label="Equipos">{lineup.team_count}</Fact>
        <Fact label="Por 100 rondas">
          {now.toFixed(1)}{' '}
          <span className="font-normal text-fg-2">
            {trendWord(now, before)} desde {before.toFixed(1)}
          </span>
        </Fact>
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

function Context({ lineup, map, siblings }: { lineup: ProLineup; map: string; siblings: readonly ProLineup[] }): ReactNode {
  const sites = lineup.plant_sites.filter((site) => site.key !== '');
  return (
    <section aria-labelledby="con-que-va" className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <h2 id="con-que-va" className="text-body font-semibold text-fg-1">
          Se tira con
        </h2>
        {lineup.with.length === 0 ? (
          <p className="text-body-sm text-fg-2">Suele ir solo: ningún otro lineup sale en los 15 segundos de alrededor.</p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {lineup.with.map((row) => (
              <li key={row.key} className="flex items-baseline gap-3 text-body-sm">
                <Link href={lineupHref(map, { id: row.key })} className="min-w-0 truncate text-fg-1 underline-offset-2 hover:underline">
                  {row.name}
                </Link>
                <span className="ml-auto shrink-0 text-fg-2 tabular-nums">{pct(row.share)} de las rondas</span>
              </li>
            ))}
          </ul>
        )}
      </div>
      {lineup.side === 'T' && sites.length > 0 ? (
        <p className="text-body-sm text-fg-2">
          Cuando hay plant, suele ir a{' '}
          <span className="font-medium text-fg-1">
            {sites[0].key} ({pct(sites[0].share)})
          </span>
          .
        </p>
      ) : null}
      {siblings.length > 0 ? (
        <div className="flex flex-col gap-2">
          <h3 className="text-body-sm font-semibold text-fg-1">Otros lineups que caen en {lineup.land_place || 'esta zona'}</h3>
          <ul className="flex flex-col gap-1.5">
            {siblings.slice(0, 6).map((other) => (
              <li key={other.id} className="flex items-baseline gap-3 text-body-sm">
                <Link href={lineupHref(map, other)} className="min-w-0 truncate text-fg-1 underline-offset-2 hover:underline">
                  {other.title}
                </Link>
                <span className="ml-auto shrink-0 text-fg-2 tabular-nums">{other.throws.recent} lanzamientos</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

function Examples({ lineup }: { lineup: ProLineup }): ReactNode {
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
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
