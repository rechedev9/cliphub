'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { RotateCcw } from 'lucide-react';
import { getLibraryTeamVeto, listLibraryTeams, type LibraryTeam } from '@/lib/api/library';
import {
  DEFAULT_MAP_POOL,
  EXTRA_POOL_MAPS,
  SERIES_FORMATS,
  type SeriesFormat,
  type SeriesSlot,
  type VetoSimulation,
} from '@/lib/api/veto';
import { matchesHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { SERIES_FORMAT_LABEL, VETO_ACTION_LABEL, VETO_BASIS_LABEL, edgeLabel, rateLabel, vetoSequence } from '@/lib/library/veto';
import { RelatedList } from '@/components/records/related-list';
import { Segmented } from '@/components/tactical/spatial/spatial-controls';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

const NO_OPPONENT = '';
const POOL_CHOICES = [...DEFAULT_MAP_POOL, ...EXTRA_POOL_MAPS];

/**
 * The veto simulator: the team is slot a, the rival slot b. Every open step
 * shows the map the rule recommends; choosing another map at a step keeps the
 * steps before it and recomputes the rest.
 */
export function TeamVeto({ teamId }: { teamId: string }): ReactNode {
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  const [opponent, setOpponent] = useState(NO_OPPONENT);
  const [format, setFormat] = useState<SeriesFormat>('bo3');
  const [first, setFirst] = useState<SeriesSlot>('a');
  const [pool, setPool] = useState<readonly string[]>(DEFAULT_MAP_POOL);
  const [done, setDone] = useState<readonly string[]>([]);
  const [result, setResult] = useState<VetoSimulation | null | 'failed'>(null);

  useEffect(() => {
    let active = true;
    listLibraryTeams()
      .then((list) => {
        if (active) setTeams(list.filter((team) => team.id !== teamId).sort((a, b) => a.name.localeCompare(b.name, 'es')));
      })
      .catch(() => {
        if (active) setTeams([]);
      });
    return () => {
      active = false;
    };
  }, [teamId]);

  const doneKey = done.join(',');
  const poolKey = pool.join(',');
  const fits = vetoSequence(format, pool.length, first) !== null;
  useEffect(() => {
    if (opponent === NO_OPPONENT || !fits) return;
    let active = true;
    setResult(null);
    getLibraryTeamVeto(teamId, {
      opponentId: opponent,
      format,
      first,
      pool: poolKey.split(','),
      done: doneKey === '' ? [] : doneKey.split(','),
    })
      .then((next) => {
        if (active) setResult(next);
      })
      .catch(() => {
        if (active) setResult('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, opponent, format, first, poolKey, doneKey, fits]);

  function togglePool(map: string): void {
    setDone([]);
    setPool((current) => (current.includes(map) ? current.filter((m) => m !== map) : [...current, map].sort()));
  }

  function choose(index: number, map: string): void {
    if (result === null || result === 'failed') return;
    setDone([...result.plan.steps.slice(0, index).map((step) => step.map), map]);
  }

  const plan = result !== null && result !== 'failed' ? result.plan : null;
  const opponentName = teams?.find((team) => team.id === opponent)?.name ?? 'Rival';
  const slotNames: Record<string, string> = { a: 'Nosotros', b: opponentName, '': 'Ninguno' };
  const minPool = { bo1: 2, bo3: 3, bo5: 5 }[format];
  let notice: ReactNode = null;
  if (opponent === NO_OPPONENT) notice = <p className="text-body-sm text-fg-2">Elige un rival para simular el veto.</p>;
  else if (!fits) {
    notice = (
      <p role="alert" className="text-body-sm text-warning">
        Un {SERIES_FORMAT_LABEL[format]} necesita entre {minPool} y 9 mapas en el pool.
      </p>
    );
  } else if (result === 'failed') {
    notice = (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo simular el veto. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  } else if (plan === null) notice = <Skeleton className="h-64 w-full" />;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        {teams === null ? (
          <Skeleton className="h-9 w-48" />
        ) : (
          <Select value={opponent} onValueChange={(value) => { setOpponent(value); setDone([]); }}>
            <SelectTrigger aria-label="Rival" className="h-9 min-w-48">
              <SelectValue placeholder="Elige el rival" />
            </SelectTrigger>
            <SelectContent>
              {teams.map((team) => (
                <SelectItem key={team.id} value={team.id}>
                  {team.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        <Segmented
          label="Formato"
          value={format}
          options={SERIES_FORMATS.map((value) => ({ value, label: SERIES_FORMAT_LABEL[value] }))}
          onChange={(value) => { setFormat(value); setDone([]); }}
        />
        <Segmented
          label="Empieza el veto"
          value={first}
          options={[{ value: 'a', label: 'Empezamos' }, { value: 'b', label: 'Empieza el rival' }]}
          onChange={(value) => { setFirst(value); setDone([]); }}
        />
        <Button type="button" size="sm" variant="ghost" disabled={done.length === 0} onClick={() => setDone([])}>
          <RotateCcw aria-hidden />
          Reiniciar
        </Button>
      </div>
      <fieldset className="flex flex-col gap-1.5">
        <legend className="mb-1 text-meta text-fg-3">Pool de mapas (editable: Valve rota el pool activo)</legend>
        <div className="flex flex-wrap gap-1.5">
          {POOL_CHOICES.map((map) => (
            <label
              key={map}
              className={cn(
                'flex cursor-pointer items-center gap-1.5 rounded-md border px-2 py-1 text-body-sm',
                pool.includes(map) ? 'border-primary text-fg-1' : 'border-border text-fg-3',
              )}
            >
              <input type="checkbox" className="accent-[var(--primary)]" checked={pool.includes(map)} onChange={() => togglePool(map)} />
              {prettyMapName(map)}
            </label>
          ))}
        </div>
      </fieldset>

      {notice ?? (plan === null ? null : (
        <>
          <RelatedList title={`Veto ${SERIES_FORMAT_LABEL[plan.format]} contra ${opponentName}`} count={plan.steps.length}>
            <ol className="flex flex-col divide-y divide-border-subtle">
              {plan.steps.map((step, index) => {
                const taken = new Set(plan.steps.slice(0, index).map((s) => s.map));
                return (
                  <li key={step.index} className="flex flex-wrap items-center gap-3 px-3 py-2 text-body-sm">
                    <span className="w-6 font-mono text-fg-3">{step.index}</span>
                    <span className={cn('w-36 truncate', step.team === 'a' ? 'text-fg-1' : 'text-fg-2')}>{slotNames[step.team]}</span>
                    <span className={cn('w-16 font-medium', step.action === 'pick' && 'text-outcome-win', step.action === 'ban' && 'text-outcome-loss')}>
                      {VETO_ACTION_LABEL[step.action]}
                    </span>
                    {step.action === 'decider' ? (
                      <span className="min-w-40 font-medium text-fg-1">{prettyMapName(step.map)}</span>
                    ) : (
                      <Select value={step.map} onValueChange={(map) => choose(index, map)}>
                        <SelectTrigger aria-label={`Mapa del paso ${step.index}`} className="h-8 min-w-40">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {plan.pool.filter((map) => !taken.has(map)).map((map) => (
                            <SelectItem key={map} value={map}>
                              {prettyMapName(map)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                    <span className="text-meta text-fg-3">{VETO_BASIS_LABEL[step.basis]}</span>
                  </li>
                );
              })}
            </ol>
          </RelatedList>
          <p className="text-body-sm text-fg-1">
            Se jugaría: <span className="font-medium">{plan.played.map(prettyMapName).join(', ')}</span>
          </p>
          <RelatedList title="Mapas del pool" count={plan.maps.length}>
            <div className="overflow-x-auto">
              <table aria-label="Mapas del pool" className="w-full border-collapse text-left text-body-sm">
                <thead className="bg-surface-2 text-fg-3">
                  <tr className="border-b border-border">
                    {['Mapa', 'Nuestras rondas', 'Nosotros', 'Sus rondas', 'Ellos', 'Ventaja', 'Sus bans', 'Sus picks'].map((header) => (
                      <th key={header} scope="col" className="h-9 px-3 font-medium whitespace-nowrap">
                        {header}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {plan.maps.map((map) => (
                    <tr key={map.map} className="border-b border-border-subtle last:border-b-0">
                      <th scope="row" className="px-3 py-2 text-left font-medium text-fg-1">
                        {prettyMapName(map.map)}
                      </th>
                      <td className="px-3 py-2 font-mono">
                        <Link href={matchesHref({ team: teamId, map: map.map })} className="hover:text-primary hover:underline">
                          {map.our_rounds_won}/{map.our_rounds} en {map.our_matches} {map.our_matches === 1 ? 'partida' : 'partidas'}
                        </Link>
                      </td>
                      <td className="px-3 py-2 font-mono">{rateLabel(map.our_rate)}</td>
                      <td className="px-3 py-2 font-mono">
                        <Link href={matchesHref({ team: opponent, map: map.map })} className="hover:text-primary hover:underline">
                          {map.their_rounds_won}/{map.their_rounds} en {map.their_matches} {map.their_matches === 1 ? 'partida' : 'partidas'}
                        </Link>
                      </td>
                      <td className="px-3 py-2 font-mono">{rateLabel(map.their_rate)}</td>
                      <td className={cn('px-3 py-2 font-mono', map.edge > 0 && 'text-outcome-win', map.edge < 0 && 'text-outcome-loss')}>{edgeLabel(map.edge)}</td>
                      <td className="px-3 py-2 font-mono">{map.their_bans}</td>
                      <td className="px-3 py-2 font-mono">{map.their_picks}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </RelatedList>
          <p className="measure-read text-meta text-fg-3">
            Regla: la tasa de rondas ganadas de cada equipo en un mapa se acerca al 50 % con un peso de {plan.model.prior_rounds} rondas, para que una
            sola partida no decida. Nuestros bans quitan el mapa con menos ventaja y nuestros picks eligen el de más. El rival hace lo contrario, salvo
            que tenga al menos {plan.model.history_min} picks o bans apuntados en sus series ({plan.opponent_history} ahora): entonces repite lo que más
            veces hizo. Los empates se resuelven por el nombre del mapa.
          </p>
        </>
      ))}
    </div>
  );
}
