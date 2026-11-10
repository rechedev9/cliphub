'use client';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Captions, Download, Search } from 'lucide-react';
import { replayHref } from '@/lib/analysis/replay-link';
import type { TacticalDocument } from '@/lib/api/tactical';
import { searchVoice } from '@/lib/api/voice';
import type { MatchVoice, VoiceCommsLine, VoiceSearchHit } from '@/lib/api/voice';
import { prettyMapName } from '@/lib/format';
import { voiceErrorText } from '@/lib/replay/voice-errors';
import { formatVoiceMs } from '@/lib/replay/voice-sync';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import { MatchTalkSummary, TalkBars, VoiceTracks } from './comms-talk';
import type { Speaker } from './comms-talk';
import type { ReplayVoice } from './use-replay-voice';

type Scope = 'round' | 'match' | 'all';

const MIN_SEARCH = 2;

function mib(bytes: number): string {
  return `${Math.round(bytes / (1024 * 1024))} MB`;
}

/** The languages whisper is asked for, as the MCP's transcribe_voice takes them. */
const LANGUAGES = [
  { value: 'es', label: 'Español' },
  { value: 'en', label: 'Inglés' },
  { value: 'pt', label: 'Portugués' },
  { value: 'fr', label: 'Francés' },
  { value: 'de', label: 'Alemán' },
  { value: 'auto', label: 'Detectar' },
] as const;

function TranscribeButton({ voice, saved }: { voice: ReplayVoice; saved: MatchVoice }): ReactNode {
  const [language, setLanguage] = useState(saved.transcript.language ?? 'es');
  return (
    <div className="flex items-center gap-1.5">
      <Select value={language} onValueChange={setLanguage}>
        <SelectTrigger aria-label="Idioma de la voz" className="h-7 w-28 text-meta">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {LANGUAGES.map((entry) => (
            <SelectItem key={entry.value} value={entry.value}>
              {entry.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button type="button" size="xs" variant="outline" loading={voice.busy} onClick={() => voice.transcribe(language)}>
        <Captions aria-hidden />
        {saved.transcript.state === 'failed' ? 'Transcribir otra vez' : 'Transcribir'}
      </Button>
    </div>
  );
}

/** Local transcription: install it once, transcribe a match, follow its progress. */
function TranscriptState({ voice, saved }: { voice: ReplayVoice; saved: MatchVoice }): ReactNode {
  const { transcriber, transcript } = saved;
  if (transcript.state === 'queued' || transcript.state === 'running') {
    const share = Math.round((transcript.progress ?? 0) * 100);
    return (
      <div className="flex flex-col gap-1">
        <span className="text-meta text-fg-2">{transcript.state === 'queued' ? 'En cola para transcribir…' : `Transcribiendo en este PC… ${share} %`}</span>
        <Progress size="xs" value={share} indeterminate={transcript.state === 'queued'} />
      </div>
    );
  }
  if (transcriber.state === 'unsupported') return <p className="text-meta text-fg-3">La transcripción local no funciona en este PC.</p>;
  if (transcriber.state === 'installing') {
    const share = transcriber.download_bytes > 0 ? Math.round(((transcriber.done_bytes ?? 0) / transcriber.download_bytes) * 100) : 0;
    return (
      <div className="flex flex-col gap-1">
        <span className="text-meta text-fg-2">Descargando la transcripción local… {share} %</span>
        <Progress size="xs" value={share} />
      </div>
    );
  }
  if (transcriber.state !== 'installed') {
    return (
      <div className="flex flex-col gap-1.5">
        <p className="text-meta text-fg-3">
          Transcribe la voz en este PC, sin enviarla a ningún sitio. Se descarga una vez ({mib(transcriber.download_bytes)}).
        </p>
        {transcriber.state === 'failed' ? (
          <p className="text-meta text-destructive">{voiceErrorText(transcriber.error_code, 'La descarga no terminó. Vuelve a intentarlo.')}</p>
        ) : null}
        <Button type="button" size="xs" variant="outline" loading={voice.busy} onClick={voice.install} className="self-start">
          <Download aria-hidden />
          {transcriber.state === 'failed' ? 'Instalar otra vez' : 'Instalar transcripción'}
        </Button>
      </div>
    );
  }
  if (transcript.state === 'ready') return null;
  return (
    <div className="flex flex-col gap-1.5">
      {transcript.state === 'failed' ? <p className="text-meta text-destructive">La transcripción falló en este PC; studio.log tiene el detalle.</p> : null}
      <TranscribeButton voice={voice} saved={saved} />
    </div>
  );
}

function LineButton({ line, active, onJump }: { line: VoiceCommsLine; active: boolean; onJump: () => void }): ReactNode {
  return (
    <button
      type="button"
      onClick={onJump}
      aria-current={active ? 'true' : undefined}
      className={cn(
        'grid w-full grid-cols-[2.25rem_minmax(0,1fr)] gap-x-1.5 rounded-sm px-1 py-0.5 text-left text-meta outline-none hover:bg-surface-3',
        active && 'bg-surface-3',
        FOCUS_RING,
      )}
    >
      <span className="font-mono tabular-nums text-fg-3">{formatVoiceMs(line.round_ms).replace(/,\d$/, '')}</span>
      <span className="min-w-0">
        <span className="font-medium text-fg-2">{line.label}: </span>
        <span className="text-fg-1">{line.text}</span>
      </span>
    </button>
  );
}

function SearchHits({ hits }: { hits: readonly VoiceSearchHit[] }): ReactNode {
  if (hits.length === 0) return <p className="text-meta text-fg-3">Ninguna partida transcrita dice eso.</p>;
  return (
    <ul className="flex flex-col gap-1">
      {hits.map((hit) => (
        <li key={`${hit.match_id}-${hit.track}-${hit.start_ms}`}>
          <Link
            href={replayHref([{ matchId: hit.match_id, round: Math.max(1, hit.round), tick: hit.tick }])}
            className={cn('block rounded-sm px-1 py-0.5 text-meta outline-none hover:bg-surface-3', FOCUS_RING)}
          >
            <span className="block truncate text-fg-3">
              {prettyMapName(hit.map)} · {hit.teams.join(' vs ')} · Ronda {hit.round}
            </span>
            <span className="font-medium text-fg-2">{hit.label}: </span>
            <span className="text-fg-1">{hit.text}</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

/**
 * The Replay's comms: transcription, the round's talk time by phase, the transcript that follows the
 * playhead and jumps to a phrase, a search over this match or every match, and the tracks.
 */
export function CommsPanel({
  voice,
  doc,
  round,
  currentTick,
  onJump,
}: {
  voice: ReplayVoice;
  doc: TacticalDocument;
  round: number;
  currentTick: number | null;
  onJump: (line: VoiceCommsLine) => void;
}): ReactNode {
  const [scope, setScope] = useState<Scope>('round');
  const [query, setQuery] = useState('');
  const { freezeOnly, setFreezeOnly } = voice;
  const [hits, setHits] = useState<VoiceSearchHit[] | null>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const state = voice.state;
  const comms = state.kind === 'ready' ? state.comms : null;
  const words = query.trim().toLowerCase();

  useEffect(() => {
    if (scope !== 'all' || words.length < MIN_SEARCH) {
      setHits(null);
      return;
    }
    const timer = window.setTimeout(() => {
      searchVoice({ q: words, limit: 100, phase: freezeOnly ? 'freezetime' : undefined })
        .then((found) => setHits(found.hits))
        .catch(() => setHits([]));
    }, 300);
    return () => window.clearTimeout(timer);
  }, [freezeOnly, scope, words]);

  const lines = useMemo(() => {
    const all = comms?.lines ?? [];
    return all.filter(
      (line) =>
        (scope !== 'round' || line.round === round)
        && (!freezeOnly || line.phase === 'freezetime')
        && (words === '' || line.text.toLowerCase().includes(words)),
    );
  }, [comms, freezeOnly, round, scope, words]);
  const activeIndex = currentTick === null || scope !== 'round' ? -1 : lines.findLastIndex((line) => line.tick <= currentTick);
  useEffect(() => {
    if (activeIndex < 0) return;
    listRef.current?.children[activeIndex]?.scrollIntoView({ block: 'nearest' });
  }, [activeIndex]);

  if (state.kind !== 'ready' || !state.voice.present) return null;
  const saved = state.voice;
  const players = doc.players.filter((player) => player.steamid64 !== '');
  const nameOf = (speaker: Speaker): string => players.find((player) => player.steamid64 === speaker.steamid64)?.name ?? speaker.label;
  const roundTalk = comms?.rounds.find((entry) => entry.round === round)?.speakers ?? [];
  const transcribed = saved.transcript.state === 'ready';

  let results: ReactNode;
  if (scope === 'all') {
    results = hits === null ? <p className="text-meta text-fg-3">Escribe al menos dos letras para buscar en todas las partidas.</p> : <SearchHits hits={hits} />;
  } else if (lines.length === 0) {
    results = <p className="text-meta text-fg-3">{words === '' && !freezeOnly ? 'Nadie dijo nada en esta ronda.' : 'Ninguna frase coincide.'}</p>;
  } else {
    results = (
      <ul ref={listRef} className="flex max-h-64 flex-col overflow-y-auto overscroll-contain [scrollbar-width:thin]">
        {lines.map((line, index) => (
          <li key={`${line.track}-${line.start_ms}`}>
            <LineButton line={{ ...line, label: nameOf(line) }} active={index === activeIndex} onJump={() => onJump(line)} />
          </li>
        ))}
      </ul>
    );
  }
  return (
    <div className="flex flex-col gap-3">
      <TranscriptState voice={voice} saved={saved} />
      <section className="flex flex-col gap-1.5" aria-label="Quién habla en la ronda">
        <h3 className="text-meta font-medium text-fg-3">Quién habla en la ronda {round}</h3>
        <TalkBars speakers={roundTalk} names={nameOf} />
      </section>
      {transcribed ? (
        <section className="flex flex-col gap-1.5" aria-label="Transcripción">
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-fg-3" aria-hidden />
            <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar en las comunicaciones" aria-label="Buscar en las comunicaciones" className="h-8 pl-8 text-body-sm" />
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Segmented
              label="Dónde buscar"
              value={scope}
              onChange={setScope}
              options={[
                { value: 'round', label: 'Ronda' },
                { value: 'match', label: 'Partida' },
                { value: 'all', label: 'Todas' },
              ]}
            />
            <label className="flex items-center gap-1.5 text-meta text-fg-2">
              <Switch checked={freezeOnly} onCheckedChange={setFreezeOnly} aria-label="Solo tiempo de compra: también silencia la voz durante la ronda" />
              Solo compra
            </label>
          </div>
          {results}
        </section>
      ) : null}
      <section className="flex flex-col gap-1.5" aria-label="Resumen de la partida">
        <h3 className="text-meta font-medium text-fg-3">En toda la partida</h3>
        <MatchTalkSummary totals={comms?.totals ?? []} names={nameOf} />
      </section>
      <section className="flex flex-col gap-1.5" aria-label="Pistas de voz">
        <h3 className="text-meta font-medium text-fg-3">Pistas</h3>
        <VoiceTracks voice={voice} tracks={saved.tracks} players={players} />
      </section>
    </div>
  );
}
