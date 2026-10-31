'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { Layers, Pause, Play, RotateCcw } from 'lucide-react';
import { getSpatialOverlay } from '@/lib/api/library';
import type { SpatialScope } from '@/lib/api/library';
import type { OverlayRound, SpatialMatchRef, SpatialResponse } from '@/lib/api/spatial';
import { RADAR_LEVELS, TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { RadarLevel, TacticalSide } from '@/lib/api/tactical';
import { matchHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { roundEvidenceHref } from '@/lib/library/rounds';
import { buyLabel, ctPatternLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import {
  LEVEL_LABEL,
  elapsedLabel,
  noteSeconds,
  noteVisibleAt,
  overlayDuration,
  overlayRoundKey,
  trackDeadAt,
  trackPositionAt,
  trackTrail,
} from '@/lib/tactical-spatial';
import { TACTICAL_REPLAY_SPEEDS } from '@/lib/tactical-timeline';
import { isMultiLevelMap, levelForAltitude, worldToRendered } from '@/lib/tactical-transform';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { AnnotationPanel, useAnnotations } from './annotation-panel';
import type { NoteItem } from './annotation-panel';
import { RadarSurface } from './radar-surface';
import type { RadarFrame, RadarSurfaceHandle } from './radar-surface';
import { Segmented } from './spatial-controls';
import { TelestratorLayer, useTelestratorDraft } from './telestrator';

/** Seconds of path drawn behind each dot. */
const TRAIL_SECONDS = 4;
/** A grenade or a kill stays on the radar this long after it happened. */
const EVENT_SECONDS = 6;
/** React re-renders the list and the notes this many times a second while playing. */
const UI_TICKS_PER_SECOND = 4;
/** World radius of a smoke, for its ring on the radar. */
const SMOKE_RADIUS = 144;

type Load = { kind: 'loading' } | { kind: 'ready'; data: SpatialResponse } | { kind: 'failed' };

function roundTitle(round: OverlayRound, matches: ReadonlyMap<string, SpatialMatchRef>, many: boolean): string {
  const match = matches.get(round.match_id);
  if (!many || match === undefined) return `Ronda ${round.round}`;
  const who = match.opponent ? `vs ${match.opponent}` : prettyMapName(match.map);
  return `${who} · ${formatShortDate(match.played_at)} · R${round.round}`;
}

function outcomeText(round: OverlayRound): string {
  if (round.side === undefined || round.winner === '') return '';
  return round.winner === round.side ? 'Ganada' : 'Perdida';
}

function patternText(round: OverlayRound): string {
  if (round.side === TACTICAL_SIDES.ct) return ctPatternLabel(round.ct_pattern);
  if (round.side === TACTICAL_SIDES.t) return tPatternLabel(round.t_pattern);
  return `${tPatternLabel(round.t_pattern)} / ${ctPatternLabel(round.ct_pattern)}`;
}

/**
 * Several rounds on one radar, every one started at its own freeze end, so the
 * same second of each round plays at once. The focused round is drawn on top
 * and is where a new note is anchored.
 */
export function OverlayView({
  scope,
  onLoaded,
}: {
  scope: SpatialScope;
  onLoaded?: (response: SpatialResponse) => void;
}): ReactNode {
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const scopeKey = JSON.stringify(scope);
  const scopeRef = useRef(scope);

  useEffect(() => {
    scopeRef.current = scope;
  });

  useEffect(() => {
    let active = true;
    setLoad({ kind: 'loading' });
    getSpatialOverlay(scopeRef.current)
      .then((data) => {
        if (!active) return;
        setLoad({ kind: 'ready', data });
        onLoaded?.(data);
      })
      .catch(() => {
        if (active) setLoad({ kind: 'failed' });
      });
    return () => {
      active = false;
    };
  }, [scopeKey, onLoaded]);

  if (load.kind === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudieron superponer las rondas. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (load.kind === 'loading') {
    return (
      <div role="status" aria-label="Superponiendo las rondas" className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <Skeleton className="aspect-square w-full" />
        <Skeleton className="h-80 w-full" />
      </div>
    );
  }
  const { data } = load;
  if (data.overlay === undefined || data.geometry === null || data.overlay.rounds.length === 0) {
    return (
      <p className="flex items-center gap-2 text-body-sm text-fg-3">
        <Layers className="size-4" aria-hidden />
        No hay rondas que superponer con este filtro.
      </p>
    );
  }
  return <OverlayPlayer key={scopeKey} data={data} />;
}

function OverlayPlayer({ data }: { data: SpatialResponse }): ReactNode {
  const overlay = data.overlay;
  const geometry = data.geometry;
  const rounds = useMemo(() => overlay?.rounds ?? [], [overlay]);
  const step = overlay?.step_seconds ?? 0.5;
  const duration = Math.max(1, overlayDuration(rounds));
  const matches = useMemo(() => new Map(data.matches.map((m) => [m.match_id, m])), [data.matches]);
  const many = matches.size > 1;

  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set());
  const [focus, setFocus] = useState<string>(rounds[0] ? overlayRoundKey(rounds[0]) : '');
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [uiTick, setUiTick] = useState(0);
  const [level, setLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const positionRef = useRef(0);
  const surfaceRef = useRef<RadarSurfaceHandle | null>(null);
  const scrubRef = useRef<HTMLInputElement>(null);
  const clockRef = useRef<HTMLSpanElement>(null);
  const draft = useTelestratorDraft();
  const matchIds = useMemo(() => [...new Set(rounds.map((round) => round.match_id))], [rounds]);
  const store = useAnnotations(matchIds);

  const byKey = useMemo(() => new Map(rounds.map((round) => [overlayRoundKey(round), round])), [rounds]);
  const focused = byKey.get(focus);
  const now = uiTick / UI_TICKS_PER_SECOND;

  const paint = useCallback(
    ({ ctx, size, style }: RadarFrame) => {
      if (geometry === null) return;
      const calibration = geometry.calibration;
      const seconds = positionRef.current;
      const perWorld = size / (calibration.size * calibration.scale);
      const dot = Math.max(3, size * 0.0085);
      const sideColor = (side: TacticalSide | undefined): string => (side === TACTICAL_SIDES.t ? style.t : style.ct);
      const ordered = rounds.filter((round) => !hidden.has(overlayRoundKey(round)));
      ordered.sort((a, b) => Number(overlayRoundKey(a) === focus) - Number(overlayRoundKey(b) === focus));
      const dim = ordered.length > 1;
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';
      for (const round of ordered) {
        const strong = !dim || overlayRoundKey(round) === focus;
        const base = strong ? 1 : 0.38;
        for (const event of round.events) {
          if (event.seconds > seconds || seconds - event.seconds > EVENT_SECONDS) continue;
          const fade = 1 - (seconds - event.seconds) / EVENT_SECONDS;
          const p = worldToRendered(calibration, event.pos[0], event.pos[1], size);
          ctx.globalAlpha = base * (0.25 + 0.55 * fade);
          if (event.kind === TACTICAL_EVENT_KINDS.kill) {
            const r = dot * 0.9;
            ctx.strokeStyle = sideColor(event.side);
            ctx.lineWidth = Math.max(1.5, size * 0.0028);
            ctx.beginPath();
            ctx.moveTo(p.x - r, p.y - r);
            ctx.lineTo(p.x + r, p.y + r);
            ctx.moveTo(p.x + r, p.y - r);
            ctx.lineTo(p.x - r, p.y + r);
            ctx.stroke();
          } else if (event.kind === TACTICAL_EVENT_KINDS.plant) {
            ctx.fillStyle = style.bomb;
            ctx.fillRect(p.x - dot, p.y - dot, dot * 2, dot * 2);
          } else {
            ctx.strokeStyle = event.kind === TACTICAL_EVENT_KINDS.molotov || event.kind === TACTICAL_EVENT_KINDS.he ? style.bomb : style.utility;
            ctx.lineWidth = Math.max(1, size * 0.002);
            ctx.beginPath();
            const radius = event.kind === TACTICAL_EVENT_KINDS.smoke ? SMOKE_RADIUS * perWorld : dot * 1.4;
            ctx.arc(p.x, p.y, radius, 0, Math.PI * 2);
            ctx.stroke();
          }
        }
        for (const track of round.tracks) {
          const color = sideColor(track.side);
          const trail = trackTrail(track, seconds, step, TRAIL_SECONDS);
          if (trail.length > 1) {
            ctx.strokeStyle = color;
            ctx.lineWidth = Math.max(1.25, size * 0.003);
            ctx.globalAlpha = base * 0.45;
            ctx.beginPath();
            trail.forEach(([x, y], i) => {
              const p = worldToRendered(calibration, x, y, size);
              if (i === 0) ctx.moveTo(p.x, p.y);
              else ctx.lineTo(p.x, p.y);
            });
            ctx.stroke();
          }
          const at = trackPositionAt(track, seconds, step);
          const index = Math.min(track.points.length - 1, Math.max(0, Math.floor(seconds / step - track.start)));
          const onLevel = levelForAltitude(calibration, track.points[index]?.[2] ?? 0) === level;
          if (at !== undefined) {
            const p = worldToRendered(calibration, at[0], at[1], size);
            ctx.globalAlpha = base * (onLevel ? 1 : 0.35);
            ctx.fillStyle = color;
            ctx.beginPath();
            ctx.arc(p.x, p.y, dot, 0, Math.PI * 2);
            ctx.fill();
            if (strong && dim && size >= 380) {
              ctx.globalAlpha = 0.9;
              ctx.fillStyle = style.text;
              ctx.font = `${Math.max(12, Math.round(size * 0.015))}px ${style.fontFamily}`;
              ctx.textBaseline = 'middle';
              ctx.fillText(track.name.slice(0, 10), p.x + dot * 1.6, p.y);
            }
          } else if (trackDeadAt(track, seconds, step)) {
            const last = track.points[track.points.length - 1];
            const p = worldToRendered(calibration, last[0], last[1], size);
            ctx.globalAlpha = base * 0.6;
            ctx.strokeStyle = color;
            ctx.lineWidth = Math.max(1.25, size * 0.0025);
            const r = dot * 0.7;
            ctx.beginPath();
            ctx.moveTo(p.x - r, p.y - r);
            ctx.lineTo(p.x + r, p.y + r);
            ctx.moveTo(p.x + r, p.y - r);
            ctx.lineTo(p.x - r, p.y + r);
            ctx.stroke();
          }
        }
      }
      ctx.globalAlpha = 1;
    },
    [focus, geometry, hidden, level, rounds, step],
  );

  const show = useCallback(
    (seconds: number) => {
      const clamped = Math.min(Math.max(seconds, 0), duration);
      positionRef.current = clamped;
      if (scrubRef.current !== null) scrubRef.current.value = String(clamped);
      if (clockRef.current !== null) clockRef.current.textContent = elapsedLabel(clamped);
      surfaceRef.current?.repaint();
      const ui = Math.floor(clamped * UI_TICKS_PER_SECOND);
      setUiTick((current) => (current === ui ? current : ui));
    },
    [duration],
  );

  useEffect(() => {
    if (!playing) return;
    let frame = 0;
    let last = performance.now();
    const loop = (time: number): void => {
      const next = positionRef.current + ((time - last) / 1000) * speed;
      last = time;
      show(next);
      if (next >= duration) {
        setPlaying(false);
        return;
      }
      frame = requestAnimationFrame(loop);
    };
    frame = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(frame);
  }, [playing, speed, duration, show]);

  const togglePlay = (): void => {
    if (!playing && positionRef.current >= duration) show(0);
    setPlaying(!playing);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>): void => {
    const target = event.target;
    const typing = target instanceof HTMLElement && (target.tagName === 'TEXTAREA' || target.tagName === 'BUTTON' || target.tagName === 'INPUT');
    if (event.key === ' ' && !typing) {
      event.preventDefault();
      togglePlay();
    }
  };

  const notes = useMemo<{ item: NoteItem; seconds: number; key: string }[]>(() => {
    const out: { item: NoteItem; seconds: number; key: string }[] = [];
    for (const note of store.notes ?? []) {
      const key = overlayRoundKey({ match_id: note.match_id, round: note.round });
      const round = byKey.get(key);
      if (round === undefined) continue;
      const seconds = noteSeconds(note, round);
      out.push({
        key,
        seconds,
        item: {
          note,
          label: `${roundTitle(round, matches, many)} · ${elapsedLabel(seconds)}`,
          active: noteVisibleAt(seconds, now) && !hidden.has(key),
          href: roundEvidenceHref(matchHref(note.match_id), note.round, note.tick),
          onSeek: () => {
            setPlaying(false);
            setFocus(key);
            setHidden((current) => {
              if (!current.has(key)) return current;
              const next = new Set(current);
              next.delete(key);
              return next;
            });
            show(seconds);
          },
        },
      });
    }
    return out.sort((a, b) => a.seconds - b.seconds);
  }, [byKey, hidden, many, matches, now, show, store.notes]);

  if (geometry === null || overlay === undefined) return null;
  const shown = notes
    .filter((entry) => entry.item.active && entry.item.note.strokes.length > 0)
    .map((entry) => ({ id: entry.item.note.id, strokes: entry.item.note.strokes, faded: entry.key !== focus }));

  const save = async (text: string): Promise<boolean> => {
    if (focused === undefined) return false;
    const rate = focused.tickrate > 0 ? focused.tickrate : 64;
    return store.create({
      match_id: focused.match_id,
      round: focused.round,
      tick: focused.tick_freeze_end + Math.round(positionRef.current * rate),
      text,
      strokes: draft.strokes,
    });
  };

  const toggleHidden = (key: string): void =>
    setHidden((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  return (
    <section aria-label="Rondas superpuestas" className="@container" onKeyDown={onKeyDown}>
      <div className="grid gap-4 @[56rem]:grid-cols-[minmax(0,1fr)_320px]">
        <div className="flex min-w-0 flex-col gap-3">
          <RadarSurface
            geometry={geometry}
            level={level}
            paint={paint}
            handleRef={surfaceRef}
            label={`${rounds.length - hidden.size} rondas superpuestas desde el final del tiempo de compra. La lista de rondas las describe en texto.`}
          >
            {(projection) => (
              <TelestratorLayer calibration={geometry.calibration} projection={projection} shown={shown} draft={draft} />
            )}
          </RadarSurface>
          <div className="mx-auto flex w-full flex-wrap items-center gap-2">
            <Button type="button" size="icon-sm" variant="outline" aria-label={playing ? 'Pausar' : 'Reproducir'} onClick={togglePlay}>
              {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="Volver al final del tiempo de compra"
              onClick={() => {
                setPlaying(false);
                show(0);
              }}
            >
              <RotateCcw aria-hidden />
            </Button>
            <input
              ref={scrubRef}
              type="range"
              min={0}
              max={duration}
              step={0.1}
              defaultValue={0}
              aria-label="Segundos desde el final del tiempo de compra"
              onChange={(event) => show(Number(event.target.value))}
              className="min-w-40 flex-1 accent-primary"
            />
            <span ref={clockRef} className="w-12 text-body-sm tabular-nums text-fg-1">
              0:00
            </span>
            <Segmented
              label="Velocidad"
              value={String(speed)}
              options={TACTICAL_REPLAY_SPEEDS.filter((s) => s >= 0.5).map((s) => ({ value: String(s), label: `${s}x` }))}
              onChange={(value) => setSpeed(Number(value))}
            />
            {isMultiLevelMap(geometry.calibration) ? (
              <Segmented
                label="Piso"
                value={level}
                options={[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] }))}
                onChange={setLevel}
              />
            ) : null}
          </div>
          <p className="text-meta text-fg-3">
            Cada ronda empieza en su final del tiempo de compra. Puntos cada {step} s, interpolados.
            {overlay.total > rounds.length ? ` Se muestran ${rounds.length} de ${overlay.total} rondas: afina el filtro para ver otras.` : ''}
            {overlay.missing_positions > 0 ? ` ${overlay.missing_positions} sin posiciones guardadas.` : ''}
          </p>
        </div>
        <aside className="flex min-w-0 flex-col gap-4">
          <RoundList
            rounds={rounds}
            matches={matches}
            many={many}
            hidden={hidden}
            focus={focus}
            now={now}
            onToggle={toggleHidden}
            onFocus={setFocus}
            onShowAll={() => setHidden(new Set())}
            onHideOthers={() => setHidden(new Set(rounds.map(overlayRoundKey).filter((key) => key !== focus)))}
          />
          <AnnotationPanel
            store={store}
            items={notes.map((entry) => entry.item)}
            draft={draft}
            anchorLabel={focused ? `Se ancla a ${roundTitle(focused, matches, many)} · ${elapsedLabel(now)}` : null}
            onCreate={save}
          />
        </aside>
      </div>
    </section>
  );
}

function RoundList({
  rounds,
  matches,
  many,
  hidden,
  focus,
  now,
  onToggle,
  onFocus,
  onShowAll,
  onHideOthers,
}: {
  rounds: readonly OverlayRound[];
  matches: ReadonlyMap<string, SpatialMatchRef>;
  many: boolean;
  hidden: ReadonlySet<string>;
  focus: string;
  now: number;
  onToggle: (key: string) => void;
  onFocus: (key: string) => void;
  onShowAll: () => void;
  onHideOthers: () => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border-strong bg-surface-1">
      <div className="flex items-center justify-between gap-2 border-b border-border-subtle px-3 py-2">
        <h3 className="text-body-sm font-medium text-fg-1">
          Rondas <span className="tabular-nums text-fg-3">{rounds.length - hidden.size}/{rounds.length}</span>
        </h3>
        <span className="flex gap-1">
          <Button type="button" size="xs" variant="ghost" onClick={onShowAll}>
            Todas
          </Button>
          <Button type="button" size="xs" variant="ghost" onClick={onHideOthers}>
            Solo la marcada
          </Button>
        </span>
      </div>
      <ul className="max-h-[38vh] overflow-y-auto overscroll-contain px-1 pb-1">
        {rounds.map((round) => {
          const key = overlayRoundKey(round);
          const isFocus = key === focus;
          const rate = round.tickrate > 0 ? round.tickrate : 64;
          const outcome = outcomeText(round);
          return (
            <li key={key} className={cn('flex items-start gap-2 rounded-md px-2 py-1.5', isFocus && 'bg-surface-3')}>
              <input
                type="checkbox"
                checked={!hidden.has(key)}
                onChange={() => onToggle(key)}
                aria-label={`Mostrar ${roundTitle(round, matches, many)}`}
                className="mt-1 accent-primary"
              />
              <div className="flex min-w-0 flex-1 flex-col">
                <button
                  type="button"
                  aria-pressed={isFocus}
                  onClick={() => onFocus(key)}
                  className="truncate text-left text-body-sm text-fg-1 outline-none hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
                >
                  {roundTitle(round, matches, many)}
                </button>
                <span className="truncate text-meta text-fg-3">
                  {round.side ? <span className={round.side === TACTICAL_SIDES.t ? 'text-side-t' : 'text-side-ct'}>{round.side} </span> : null}
                  {round.buy ? `${buyLabel(round.buy)} · ` : ''}
                  {patternText(round)} · {siteLabel(round.site)}
                  {outcome ? ` · ${outcome}` : ''}
                </span>
              </div>
              <Link
                href={roundEvidenceHref(matchHref(round.match_id), round.round, round.tick_freeze_end + Math.round(Math.min(now, round.duration_seconds) * rate))}
                className="shrink-0 pt-0.5 text-meta text-fg-3 hover:text-fg-1 hover:underline"
              >
                Abrir
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
