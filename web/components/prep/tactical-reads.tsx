import type { ReactNode } from 'react';
import Link from 'next/link';
import { Play } from 'lucide-react';
import { ATTACK_SITES, ATTACK_TEMPOS } from '@/lib/api/attack';
import type { AttackBand, AttackCell, AttackExecute, AttackMatchup, TeamAttack } from '@/lib/api/attack';
import { BAND_LABEL, DEFAULT_TRAMO, attackHref, siteLabel, siteZones, tempoLabel } from '@/lib/analysis/attack';
import { secondsLabel } from '@/lib/analysis/prep';
import { placeLabel } from '@/lib/tactical-labels';
import { MIN_RELIABLE_SAMPLE } from '@/lib/api/tactical';
import { cn } from '@/lib/utils';
import { SlashTitle } from '@/components/records/document-page';

const pct = (value: number): string => `${Math.round(value)} %`;

/** The four matchups EDGE always shows, in its order; any other matchup the rounds hold follows. */
const CORE_MATCHUPS: readonly (readonly [AttackBand, AttackBand])[] = [
  ['pistol', 'pistol'],
  ['half', 'full'],
  ['full', 'half'],
  ['full', 'full'],
];

function matchupName(buy: AttackBand, opponentBuy: AttackBand): string {
  return `${BAND_LABEL[buy]} contra ${BAND_LABEL[opponentBuy].toLowerCase()}`;
}

function orderedMatchups(attack: TeamAttack): { buy: AttackBand; opponentBuy: AttackBand; matchup: AttackMatchup | undefined }[] {
  const core = CORE_MATCHUPS.map(([buy, opponentBuy]) => ({
    buy,
    opponentBuy,
    matchup: attack.matchups.find((entry) => entry.buy === buy && entry.opponent_buy === opponentBuy),
  }));
  const rest = attack.matchups
    .filter((entry) => entry.rounds > 0 && !CORE_MATCHUPS.some(([buy, opponentBuy]) => buy === entry.buy && opponentBuy === entry.opponent_buy))
    .map((entry) => ({ buy: entry.buy, opponentBuy: entry.opponent_buy, matchup: entry }));
  return [...core, ...rest];
}

function ReadCell({ cell, href }: { cell: AttackCell | undefined; href: string | undefined }): ReactNode {
  const rounds = cell?.rounds ?? 0;
  const text = `${pct(cell?.share.pct ?? 0)} (${rounds})`;
  return (
    <td className={cn('px-3 py-2 text-center align-middle tabular-nums', rounds < MIN_RELIABLE_SAMPLE ? 'text-fg-3' : 'text-fg-1')}>
      {href === undefined ? (
        <span className="font-semibold">{text}</span>
      ) : (
        <Link href={href} className="font-semibold hover:text-primary hover:underline">
          {text}
        </Link>
      )}
      {cell !== undefined && rounds > 0 ? <span className="block text-meta font-normal text-fg-3">gana {pct(cell.win_rate.pct)}</span> : null}
    </td>
  );
}

/** One EDGE read: a row per site (or tempo), a column per buy matchup, "38 % (3)" cells and the rounds read at the foot. */
function ReadTable({ attack, kind }: { attack: TeamAttack; kind: 'sites' | 'tempo' }): ReactNode {
  const keys: readonly string[] = kind === 'sites' ? ATTACK_SITES : ATTACK_TEMPOS;
  const label = kind === 'sites' ? siteLabel : tempoLabel;
  const columns = orderedMatchups(attack);
  const total = kind === 'sites' ? attack.sites : attack.tempo;
  const linkOf = (cell: AttackCell | undefined, key: string, what: string): string | undefined =>
    cell === undefined || cell.rounds === 0
      ? undefined
      : attackHref(attack, { refs: cell.refs, what: `${what} · ${label(key)}`, zones: kind === 'sites' ? siteZones(key) : [] });
  return (
    <div className="overflow-x-auto">
      <table aria-label={kind === 'sites' ? 'A contra B' : 'Ritmo de bombsite'} className="w-full border-separate border-spacing-y-1 text-body-sm">
        <thead>
          <tr className="text-meta font-medium tracking-wider text-fg-3 uppercase">
            <th scope="col" className="px-3 py-1 text-left font-medium" />
            <th scope="col" className="px-3 py-1 font-medium whitespace-nowrap">
              Todas las rondas
            </th>
            {columns.map(({ buy, opponentBuy }) => (
              <th key={`${buy}|${opponentBuy}`} scope="col" className="px-3 py-1 font-medium whitespace-nowrap">
                {matchupName(buy, opponentBuy)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {keys.map((key) => (
            <tr key={key} className="bg-surface-2">
              <th scope="row" className="px-3 py-2 text-center text-body-sm font-semibold whitespace-nowrap text-fg-1">
                {label(key)}
              </th>
              <ReadCell cell={total.find((cell) => cell.key === key)} href={linkOf(total.find((cell) => cell.key === key), key, 'Todas las rondas')} />
              {columns.map(({ buy, opponentBuy, matchup }) => {
                const cell = (kind === 'sites' ? matchup?.sites : matchup?.tempo)?.find((entry) => entry.key === key);
                return <ReadCell key={`${buy}|${opponentBuy}`} cell={cell} href={linkOf(cell, key, matchupName(buy, opponentBuy))} />;
              })}
            </tr>
          ))}
          <tr className="text-meta text-fg-3">
            <th scope="row" className="px-3 py-1.5 text-center font-medium tracking-wider whitespace-nowrap uppercase">
              Rondas totales
            </th>
            <td className="px-3 py-1.5 text-center tabular-nums">{attack.round_count}</td>
            {columns.map(({ buy, opponentBuy, matchup }) => (
              <td key={`${buy}|${opponentBuy}`} className="px-3 py-1.5 text-center tabular-nums">
                {matchup?.rounds ?? 0}
              </td>
            ))}
          </tr>
        </tbody>
      </table>
    </div>
  );
}

function PlayLink({ href, label }: { href: string | undefined; label: string }): ReactNode {
  if (href === undefined) return null;
  return (
    <Link
      href={href}
      aria-label={label}
      title={label}
      className="inline-flex size-6 shrink-0 items-center justify-center rounded-md border border-primary text-primary hover:bg-primary/14"
    >
      <Play aria-hidden className="size-3" />
    </Link>
  );
}

function Defaults({ attack }: { attack: TeamAttack }): ReactNode {
  if (attack.defaults.length === 0) return <p className="text-body-sm text-fg-2">Sin defaults con posiciones en estas rondas.</p>;
  return (
    <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 print:grid-cols-3">
      {attack.defaults.map((entry) => {
        const zones = entry.zones.map((zone) => zone.zone);
        const what = `default ${entry.formation}`;
        return (
          <li key={entry.formation} className="studio-panel flex items-start justify-between gap-2 px-3 py-2.5">
            <div className="flex min-w-0 flex-col gap-0.5">
              <span className="text-body-sm font-semibold text-fg-1">A, medio y B: {entry.formation}</span>
              <span className="text-meta text-fg-2 tabular-nums">
                Usado {pct(entry.share.pct)} · Victorias {pct(entry.win_rate.pct)}
              </span>
              <span className="truncate text-meta text-fg-3">{entry.zones.map((zone) => zone.label || placeLabel(zone.zone)).join(', ') || 'Sin zona fija'}</span>
            </div>
            <PlayLink href={attackHref(attack, { refs: entry.refs, what, zones, tramo: DEFAULT_TRAMO })} label={`Ver en el Replay: ${what}`} />
          </li>
        );
      })}
    </ul>
  );
}

function executeWhat(entry: AttackExecute): string {
  const route = entry.route === '' ? '' : ` por ${entry.route_label || placeLabel(entry.route)}`;
  return `${BAND_LABEL[entry.buy]} contra ${BAND_LABEL[entry.opponent_buy].toLowerCase()} · ${siteLabel(entry.site)}${route}`;
}

function ExecuteRow({ attack, entry, withBuy }: { attack: TeamAttack; entry: AttackExecute; withBuy: boolean }): ReactNode {
  const what = executeWhat(entry);
  const route = entry.route === '' ? '' : ` por ${entry.route_label || placeLabel(entry.route)}`;
  const zones = [...(entry.route === '' ? [] : [entry.route]), ...siteZones(entry.site)];
  return (
    <li className="flex items-center justify-between gap-3 bg-surface-2 px-3 py-2">
      <div className="flex min-w-0 flex-col">
        <span className="truncate text-body-sm font-semibold text-fg-1">{withBuy ? what : `${siteLabel(entry.site)}${route}`}</span>
        {entry.contact_seconds === null ? null : <span className="text-meta text-fg-3">Llega en {secondsLabel(entry.contact_seconds)}</span>}
      </div>
      <div className="flex shrink-0 items-center gap-4 text-meta text-fg-2 tabular-nums">
        <span>Usado {pct(entry.share.pct)}</span>
        <span>Rush {pct(entry.rushes.pct)}</span>
        <span>Planta {pct(entry.plants.pct)}</span>
        <span>Victorias {pct(entry.win_rate.pct)}</span>
        <PlayLink href={attackHref(attack, { refs: entry.refs, what, zones })} label={`Ver en el Replay: ${what}`} />
      </div>
    </li>
  );
}

const isFullVsFull = (entry: AttackExecute): boolean => entry.buy === 'full' && entry.opponent_buy === 'full';

/** The EDGE "Lecturas tácticas": A contra B and bombsite tempo by buy matchup, preferred defaults and full buy against full buy. */
export function TacticalReads({ attack }: { attack: TeamAttack }): ReactNode {
  const fullBuys = attack.executes.filter(isFullVsFull);
  return (
    <section aria-label="Lecturas tácticas" className="flex flex-col gap-4 print:break-inside-avoid">
      <SlashTitle>Lecturas tácticas</SlashTitle>
      <div className="flex flex-col gap-1.5">
        <h3 className="text-body font-semibold text-fg-1">A contra B</h3>
        <p className="text-meta text-fg-3">Qué parte de las rondas va a cada sitio por cruce de compras, con las rondas leídas entre paréntesis. Cada cifra abre sus rondas con la zona marcada.</p>
        <ReadTable attack={attack} kind="sites" />
      </div>
      <div className="flex flex-col gap-1.5">
        <h3 className="text-body font-semibold text-fg-1">Ritmo de bombsite</h3>
        <p className="text-meta text-fg-3">Rush, estrategia preparada, media ronda o sin pisar sitio por cruce de compras.</p>
        <ReadTable attack={attack} kind="tempo" />
      </div>
      <div className="flex flex-col gap-1.5">
        <h3 className="text-body font-semibold text-fg-1">Defaults preferidos</h3>
        <p className="text-meta text-fg-3">Dónde están a los 25 s en las rondas lentas.</p>
        <Defaults attack={attack} />
      </div>
      {fullBuys.length === 0 ? null : (
        <div className="flex flex-col gap-1.5">
          <h3 className="text-body font-semibold text-fg-1">Compra completa contra completa</h3>
          <ul className="flex flex-col gap-1">
            {fullBuys.map((entry) => (
              <ExecuteRow key={`${entry.site}|${entry.route}`} attack={attack} entry={entry} withBuy={false} />
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

/** The executes that are not full buy against full buy; those sit in the Lecturas tácticas. */
export function OtherExecutes({ attack }: { attack: TeamAttack }): ReactNode {
  const others = attack.executes.filter((entry) => !isFullVsFull(entry));
  if (others.length === 0) return <p className="text-body-sm text-fg-2">Sin ejecuciones a un sitio con otras compras en estas rondas.</p>;
  return (
    <ul className="flex flex-col gap-1">
      {others.map((entry) => (
        <ExecuteRow key={`${entry.buy}|${entry.opponent_buy}|${entry.site}|${entry.route}`} attack={attack} entry={entry} withBuy />
      ))}
    </ul>
  );
}
