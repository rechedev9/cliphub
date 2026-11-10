'use client';

import { useState, type ReactNode } from 'react';
import { Copy } from 'lucide-react';
import { botReplayGrenadeLabel, launchBotReplay, sidePlayers, type BotReplayOutcome, type BotReplayResult, type BotReplaySide } from '@/lib/api/bot-replay';
import type { TacticalDocument, TacticalRound } from '@/lib/api/tactical';
import { teamKeyOnSide } from '@/lib/library/rounds';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Segmented } from '@/components/ui/segmented';

const SIDES: readonly BotReplaySide[] = ['CT', 'T'];

/** "Practicar contra esta ronda": pick a side and a player, then CS2 opens with the other side as bots. */
export function BotReplayDialog({
  matchId,
  doc,
  round,
  onClose,
}: {
  matchId: string;
  doc: TacticalDocument;
  round: TacticalRound;
  onClose: () => void;
}): ReactNode {
  const [side, setSide] = useState<BotReplaySide>('CT');
  const [chosen, setChosen] = useState('');
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<BotReplayOutcome | null>(null);
  const teamName = (which: BotReplaySide): string => doc.teams.find((team) => team.key === teamKeyOnSide(doc, round, which))?.name ?? which;
  const players = sidePlayers({ doc, round, side });
  const replace = players.some((player) => player.steamid64 === chosen) ? chosen : (players[0]?.steamid64 ?? '');
  const rivals = sidePlayers({ doc, round, side: side === 'CT' ? 'T' : 'CT' });

  async function launch(): Promise<void> {
    setBusy(true);
    setOutcome(await launchBotReplay({ matchId, round: round.number, side, replaceSteamId: replace }));
    setBusy(false);
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-2xl" showCloseButton={!busy}>
        <DialogHeader>
          <DialogTitle>Practicar contra la ronda {round.number}</DialogTitle>
          <DialogDescription>
            CS2 se abre en un servidor local con un bot por cada jugador del otro equipo. Los bots están quietos y no disparan: en cada momento clave
            de la ronda aparecen donde estaba cada rival, y el bot de un rival que ya ha muerto espera en su spawn.
          </DialogDescription>
        </DialogHeader>
        {outcome?.kind === 'ok' ? (
          <LaunchedSummary result={outcome.result} />
        ) : (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Juegas de
              <Segmented
                label="Juegas de"
                value={side}
                options={SIDES.map((value) => ({ value, label: `${value}: ${teamName(value)}` }))}
                onChange={(next) => setSide(next)}
              />
            </div>
            {players.length === 0 ? null : (
              <div className="flex flex-col gap-1.5 text-body-sm text-fg-2">
                Sales como
                <Segmented
                  label="Sales como"
                  value={replace}
                  options={players.map((player) => ({ value: player.steamid64, label: player.name }))}
                  onChange={(next) => setChosen(next)}
                />
              </div>
            )}
            <p className="text-body-sm text-fg-1">
              {rivals.length === 0
                ? 'El otro equipo no tiene jugadores en esta ronda.'
                : `Bots: ${rivals.map((player) => player.name).join(', ')}.`}
            </p>
            <p className="text-body-sm text-fg-2">
              Los bots no repiten el recorrido, no miran hacia donde miraba el rival y no lanzan granadas: CS2 no lo permite sin un plugin de
              servidor. Las teclas del teclado numérico sustituyen a lo que tuvieras asignado en ellas.
            </p>
            {outcome === null ? null : <FailureText outcome={outcome} />}
          </div>
        )}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
            {outcome?.kind === 'ok' ? 'Cerrar' : 'Cancelar'}
          </Button>
          {outcome?.kind === 'ok' ? null : (
            <Button type="button" disabled={busy || rivals.length === 0} onClick={() => void launch()}>
              {busy ? 'Abriendo CS2' : 'Abrir CS2'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Limits({ limits }: { limits: string[] }): ReactNode {
  if (limits.length === 0) return null;
  return (
    <div className="rounded-md border border-border bg-surface-2 px-3 py-2 text-body-sm text-fg-2">
      <p className="text-fg-1">Lo que CS2 no deja hacer sin un plugin de servidor</p>
      <ul className="list-disc pl-5">
        {limits.map((limit) => (
          <li key={limit}>{limit}</li>
        ))}
      </ul>
    </div>
  );
}

function FailureText({ outcome }: { outcome: BotReplayOutcome }): ReactNode {
  let text = '';
  switch (outcome.kind) {
    case 'cs2_not_found':
      text = 'No se encontró CS2 en este PC. Instálalo desde Steam y vuelve a intentarlo.';
      break;
    case 'no_positions':
      text = 'Esta ronda no tiene posiciones guardadas. Vuelve a analizar la demo.';
      break;
    case 'offline':
      text = 'El servicio local no responde. Comprueba que ClipHub Studio sigue abierto.';
      break;
    case 'failed':
      text = `No se pudo preparar la práctica. ${outcome.message}`;
      break;
    default:
      return null;
  }
  return <p role="alert" className="text-body-sm text-destructive">{text}</p>;
}

function LaunchedSummary({ result }: { result: BotReplayResult }): ReactNode {
  const [copied, setCopied] = useState(false);
  const keys = [
    { key: result.keys.restart, text: 'coloca a los bots en el momento 1 (las dos primeras veces prepara el servidor y crea los bots)' },
    { key: result.keys.next, text: 'siguiente momento de la ronda' },
    { key: result.keys.start, text: 'otra salida' },
    { key: result.keys.base, text: `si los bots no se colocan, cambia su número y vuelve a pulsar ${result.keys.restart}` },
    { key: result.keys.prepare, text: 'prepara el servidor otra vez' },
    ...result.lineups.map((lineup) => ({ key: lineup.key, text: `${botReplayGrenadeLabel(lineup.kind)} de ${lineup.player} (${lineup.seconds.toFixed(1)} s)` })),
  ];
  return (
    <div role="status" className="flex flex-col gap-3 text-body-sm text-fg-1">
      {result.alreadyRunning || !result.launched ? (
        <div className="flex flex-col gap-2">
          <p>CS2 ya está abierto. Escribe esto en su consola para cargar la ronda:</p>
          <span className="flex flex-wrap items-center gap-2">
            <code className="rounded-sm bg-surface-3 px-2 py-1 font-mono text-fg-1">{result.execCommand}</code>
            <Button
              type="button"
              variant="ghost"
              size="xs"
              onClick={() => {
                void navigator.clipboard.writeText(result.execCommand).then(() => setCopied(true)).catch(() => setCopied(false));
              }}
            >
              <Copy aria-hidden />
              {copied ? 'Copiado' : 'Copiar'}
            </Button>
          </span>
        </div>
      ) : (
        <p>CS2 se está abriendo en {result.map}. Si Steam pide confirmación, acéptala.</p>
      )}
      <p>
        Cuando estés en el mapa, pulsa {result.keys.restart} tres veces: la primera prepara el servidor, la segunda crea los bots y la tercera
        los coloca en el momento 1.
      </p>
      {result.bots.length === 0 ? null : (
        <p className="text-fg-2">
          Bots {result.botSide}, por orden: {result.bots.map((bot) => bot.name).join(', ')}.
        </p>
      )}
      {result.startsAs === '' ? null : <p className="text-fg-2">Sales como {result.startsAs}.</p>}
      <table className="w-full text-body-sm">
        <caption className="sr-only">Teclas del teclado numérico</caption>
        <tbody>
          {keys.map((row) => (
            <tr key={`${row.key}:${row.text}`} className="border-t border-border-subtle">
              <th scope="row" className="w-32 px-2 py-1 text-left font-mono font-normal text-fg-1">{row.key}</th>
              <td className="px-2 py-1 text-fg-2">{row.text}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <ol className="flex max-h-[30vh] flex-col gap-1 overflow-y-auto">
        {result.moments.map((moment, index) => (
          <li key={`${moment.seconds}:${moment.label}`} className="tabular-nums text-fg-2">
            <span className="text-fg-1">{index + 1}. {moment.label}</span> a {moment.seconds.toFixed(1)} s
            {moment.notes.length === 0 ? null : <span className="text-fg-3">: {moment.notes.join(', ')}</span>}
          </li>
        ))}
      </ol>
      <Limits limits={result.limits} />
    </div>
  );
}
