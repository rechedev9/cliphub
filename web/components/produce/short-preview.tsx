'use client';

import { Pause, Play } from 'lucide-react';
import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react';
import { affiliateDisplayLabel, stylesForFamily, type EditConfig, type Preset } from '@/lib/api/types';
import { engineMapName, radarImageUrl } from '@/lib/api/game-assets';
import { brandImageUrl, type BrandCorner } from '@/lib/brand-kit';
import { CUSTOM_HUD_THEMES, customHudTheme } from '@/lib/custom-hud';
import { prettyMapName } from '@/lib/format';
import { verticalHudPreviewUrl } from '@/lib/hud-themes';
import { previewTexts } from '@/lib/overlay-language';
import { previewFrame, previewTimeline, portraitPreview, type PreviewFrame } from '@/lib/produce/short-preview-timeline';
import { coerceRadarOpacity, radarAutoPosition, RADAR_DEFAULT_OPACITY, type RadarSize } from '@/lib/radar-overlay';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { useBrandKit } from '@/components/clips/brand-kit-editor';
import { PresetScene, presetHudMode } from './preset-preview';

type Size = { w: number; h: number };
const RADAR_SHARE: Record<RadarSize, number> = { small: 0.26, medium: 0.32, large: 0.4 };
const SHADOW = { filter: 'drop-shadow(3px 3px 0 rgb(0 0 0 / 0.85))' };
const FONT = 'var(--font-display)';
const SUBSCRIBE_AT = 0.35;
const SUBSCRIBE_SECONDS = 4.5;
const SUBSCRIBE_LIKED_AT = 0.85;
const SUBSCRIBE_DONE_AT = 2;

/** internal/editor fitFontSize: the largest size, in steps of 2 down to 32, whose rough width fits. */
function fitFontSize(text: string, { base, safeWidth }: { base: number; safeWidth: number }): number {
  let size = base;
  while (size > 32 && text.length * size * 0.58 > safeWidth) size -= 2;
  return size;
}

export type ShortPreviewProps = {
  preset: Preset;
  /** The edit as it is sent, after the style's limits. */
  edit: EditConfig;
  map: string;
  player: string;
  /** The device frame around the screen. */
  frameClassName?: string;
  className?: string;
};

/** The example Short, played on a loop with the chosen effect, transition and every text and graphic, as the render places them. */
export function ShortPreview({ preset, edit, map, player, frameClassName, className }: ShortPreviewProps): ReactNode {
  const timeline = useMemo(() => previewTimeline(edit), [edit]);
  const [time, setTime] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [onScreen, setOnScreen] = useState(true);
  const root = useRef<HTMLDivElement>(null);
  const { kit } = useBrandKit();
  const t = time % timeline.duration;
  const frame = previewFrame(timeline, t);
  const portrait = portraitPreview(edit);
  const size: Size = portrait ? { w: 1080, h: 1920 } : { w: 1920, h: 1080 };
  const firstKill = timeline.kills[0] ?? 0;
  // A job's match carries the display name ("Mirage"); the radar route takes the engine name.
  const radarMap = engineMapName(map);

  useEffect(() => {
    // Reduced motion opens paused on the first kill, so the effect still shows; play stays one click away.
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduced) setTime(firstKill);
    else setPlaying(true);
  }, [firstKill]);

  useEffect(() => {
    const element = root.current;
    if (!element) return;
    const observer = new IntersectionObserver(([entry]) => setOnScreen(entry?.isIntersecting ?? true));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  // Off screen the loop stops, so editing the options below does not redraw the preview every frame.
  useEffect(() => {
    if (!playing || !onScreen) return;
    let last = performance.now();
    let request = requestAnimationFrame(function tick(now) {
      const step = (now - last) / 1000;
      last = now;
      setTime((current) => (current + step) % timeline.duration);
      request = requestAnimationFrame(tick);
    });
    return () => cancelAnimationFrame(request);
  }, [playing, onScreen, timeline.duration]);

  const hud = presetHudMode(preset);
  const centered = portrait && !edit.deathReel && (edit.centeredKillfeed === true || edit.verticalHud === true);
  const cornerFeed = hud === 'gameplay' || hud === 'deathnotices';
  const affiliate = stylesForFamily(edit.keyDropFamily ?? '').find((entry) => entry.id === edit.keyDropStyle);
  // generatedKeyDropEffect: a missing or zero end runs the plate to the end of the Short.
  const affiliateEnd = edit.keyDropEndSeconds !== undefined && edit.keyDropEndSeconds > 0 ? edit.keyDropEndSeconds : timeline.duration;
  const affiliateOn = affiliate !== undefined && t >= (edit.keyDropStartSeconds ?? 0) && t < affiliateEnd;
  const bannerOn = edit.brandBanner === true && kit?.banner !== undefined
    && t >= kit.banner_start_seconds && t < (kit.banner_end_seconds ?? timeline.duration);
  const subscribeStart = timeline.duration * SUBSCRIBE_AT;
  const subscribeOn = edit.subscribeBanner === true && t >= subscribeStart && t < subscribeStart + SUBSCRIBE_SECONDS;
  const affiliateWidth = Math.min(720, Math.max(280, size.w * 0.55)) / size.w;

  return (
    <div ref={root} className={cn('flex flex-col items-center gap-2', className)}>
      <div aria-hidden className={cn('w-full', frameClassName)}>
      <div className={cn('relative w-full overflow-hidden rounded-[1.25rem] bg-surface-0 [container-type:inline-size]', portrait ? 'aspect-[9/16]' : 'aspect-video')}>
        <svg viewBox={`0 0 ${size.w} ${size.h}`} className="absolute inset-0 size-full">
          <Scene preset={preset} edit={edit} frame={frame} size={size} progress={timeline.replay ? (t - timeline.replay.start) / (timeline.replay.end - timeline.replay.start) : 0} />
          {frame.flash ? <rect width={size.w} height={size.h} fill={frame.flash.color} opacity={frame.flash.opacity} /> : null}
          {edit.radar && radarMap ? <Radar edit={edit} map={radarMap} size={size} /> : null}
          {portrait && edit.verticalHud && !edit.deathReel && frame.verticalHud ? (
            <image href={verticalHudPreviewUrl((customHudTheme(edit.verticalHudTheme) ?? CUSTOM_HUD_THEMES[0])?.id ?? '', edit.overlayLanguage)} x={(size.w - 628) / 2} y={132} width={628} height={186} />
          ) : null}
          {centered || cornerFeed ? <Notices count={frame.notices} player={player} centered={centered} size={size} /> : null}
          {kit?.logo && edit.brandWatermark ? <Watermark href={brandImageUrl(kit.logo)} corner={kit.watermark_corner} opacity={kit.watermark_opacity} size={size} /> : null}
          {kit?.banner && bannerOn ? (
            <image href={brandImageUrl(kit.banner)} x={size.w * (portrait ? 0.1 : 0.275)} y={size.h * 0.8} width={size.w * (portrait ? 0.8 : 0.45)} height={size.h * 0.12} preserveAspectRatio="xMidYMid meet" />
          ) : null}
          {subscribeOn ? <SubscribeCard logo={kit?.logo ? brandImageUrl(kit.logo) : undefined} name={kit?.channel_name} size={size} liked={t >= subscribeStart + SUBSCRIBE_LIKED_AT} subscribed={t >= subscribeStart + SUBSCRIBE_DONE_AT} /> : null}
          <Texts edit={edit} frame={frame} size={size} player={player} map={map} />
        </svg>
        {affiliate && affiliateOn ? (
          <div className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2" style={{ top: `${(edit.keyDropPositionY ?? 0.86) * 100}%`, width: `${affiliateWidth * 100}%` }}>
            <img src={affiliate.preview} alt="" className="block h-auto w-full drop-shadow-[0_4px_12px_rgba(0,0,0,0.55)]" />
            {affiliate.coverClass ? <span className={`absolute ${affiliate.textClass} ${affiliate.coverClass}`} /> : null}
            <span className={`absolute flex items-center justify-center truncate text-center font-display font-black leading-none text-white drop-shadow-[0_1px_2px_rgba(0,0,0,0.95)] ${affiliate.textClass}`} style={{ fontSize: '3.4cqw' }}>
              {affiliateDisplayLabel(edit.keyDropFamily ?? '', edit.keyDropStyle ?? '', edit.keyDropCode ?? '')}
            </span>
          </div>
        ) : null}
      </div>
      </div>
      <div className="flex w-full min-w-0 items-center gap-2">
        <Button type="button" variant="ghost" size="icon-xs" aria-label={playing ? 'Pausar la vista previa' : 'Reproducir la vista previa'} onClick={() => setPlaying((value) => !value)}>
          {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
        </Button>
        <input type="range" min={0} max={Math.round(timeline.duration * 100)} value={Math.round(t * 100)} aria-label="Momento de la vista previa" aria-valuetext={frame.phase}
          onChange={(event) => { setPlaying(false); setTime(Number(event.target.value) / 100); }}
          className="h-1 min-w-0 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-format-short" />
      </div>
      <p className="min-h-[1.25rem] text-center text-meta font-semibold text-fg-1">{frame.phase}</p>
    </div>
  );
}

/** The style's scene, moved by the kill effect and the transition; world overlays move with it. */
function Scene({ preset, edit, frame, size, progress }: { preset: Preset; edit: EditConfig; frame: PreviewFrame; size: Size; progress: number }): ReactNode {
  const chromaId = useId();
  // The replay is slower: a gentle push-in across it instead of the kill's snap.
  const zoom = frame.clip === 'replay' ? 1 + 0.06 * progress : frame.zoom;
  let filter: string | undefined;
  if (frame.grade) filter = 'contrast(1.08) saturate(1.18)';
  else if (frame.clip === 'replay') filter = 'saturate(0.55) brightness(0.92)';
  const cx = size.w / 2;
  const cy = size.h / 2;
  return (
    <>
      <defs>
        <filter id={chromaId} x="-5%" y="0" width="110%" height="100%">
          <feColorMatrix in="SourceGraphic" type="matrix" values="1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 1 0" result="red" />
          <feOffset in="red" dx={frame.glitch} result="redShift" />
          <feColorMatrix in="SourceGraphic" type="matrix" values="0 0 0 0 0 0 1 0 0 0 0 0 1 0 0 0 0 0 1 0" result="cyan" />
          <feOffset in="cyan" dx={-frame.glitch} result="cyanShift" />
          <feBlend in="redShift" in2="cyanShift" mode="screen" />
        </filter>
      </defs>
      <g filter={frame.glitch > 0 ? `url(#${chromaId})` : undefined}>
        <g transform={`translate(${cx + frame.shakeX} ${cy + frame.shakeY}) scale(${zoom}) translate(${-cx} ${-cy})`}
          style={filter ? { filter } : undefined}>
          <svg width={size.w} height={size.h} viewBox="0 0 90 160" preserveAspectRatio="xMidYMid slice">
            <g transform={frame.clip === 'b' ? 'translate(90 0) scale(-1 1)' : undefined}>
              <PresetScene preset={preset} feed={false} />
              <g fill="#0b121b">
                <circle cx="46.5" cy="84" r="2.3" />
                <path d="M43.5 87H49.5L50.5 98H42.5Z" />
              </g>
              {edit.playerLabels ? (
                <g>
                  <text x="46.5" y="78" textAnchor="middle" fontSize="2.6" fontWeight="700" fill="#f4b873" fontFamily={FONT}>rival</text>
                  <rect x="42" y="79" width="9" height="1.1" fill="#0b121b" />
                  <rect x="42" y="79" width="6.3" height="1.1" fill="#4ade80" />
                </g>
              ) : null}
              {edit.grenadeTrails ? <path d="M70 150Q40 40 31 96" fill="none" stroke="#ffd166" strokeWidth=".8" strokeDasharray="2 1.4" /> : null}
            </g>
          </svg>
        </g>
      </g>
    </>
  );
}

/** ClipHub's radar of the demo's map, in internal/radaroverlay's corner and size. */
function Radar({ edit, map, size }: { edit: EditConfig; map: string; size: Size }): ReactNode {
  const short = Math.min(size.w, size.h);
  const side = short * RADAR_SHARE[edit.radarSize ?? 'medium'];
  const margin = short * 0.04;
  const position = edit.radarPosition ?? radarAutoPosition(edit.format);
  const portrait = size.h > size.w;
  const top = portrait ? 300 * size.h / 1920 : margin;
  const bottom = portrait ? size.h - size.h * 0.14 : size.h - margin;
  const x = position.endsWith('right') ? size.w - side - margin : margin;
  const y = position.startsWith('bottom') ? bottom - side : top;
  return <image href={radarImageUrl(map)} x={x} y={y} width={side} height={side} opacity={coerceRadarOpacity(edit.radarOpacity) ?? RADAR_DEFAULT_OPACITY} />;
}

function Watermark({ href, corner, opacity, size }: { href: string; corner: BrandCorner; opacity: number; size: Size }): ReactNode {
  const short = Math.min(size.w, size.h);
  const w = short * 0.16;
  const h = short * 0.1;
  const margin = short * 0.04;
  const bottom = size.h > size.w ? size.h * 0.14 : margin;
  const right = corner.endsWith('right');
  const atBottom = corner.startsWith('bottom');
  return <image href={href} x={right ? size.w - w - margin : margin} y={atBottom ? size.h - h - bottom : margin} width={w} height={h} opacity={opacity}
    preserveAspectRatio={`${right ? 'xMax' : 'xMin'}${atBottom ? 'YMax' : 'YMin'} meet`} />;
}

type SubscribeCardProps = { logo?: string; name?: string; size: Size; liked: boolean; subscribed: boolean };

const SUBSCRIBE_THUMB = 'M2 10h4v11H2zM8 10.4l4.3-6.9c.45-.72 1.45-.86 2.07-.27.43.4.6 1.02.44 1.59L13.7 9H20a2 2 0 0 1 1.97 2.35l-1.3 7.6A2.5 2.5 0 0 1 18.2 21H8z';

function SubscribeCard({ logo, name, size, liked, subscribed }: SubscribeCardProps): ReactNode {
  const portrait = size.h > size.w;
  const s = portrait
    ? { height: 100, pad: 14, gap: 16, button: 72, font: 34, pill: 330, pillFont: 30, middle: 1368 }
    : { height: 76, pad: 10, gap: 12, button: 56, font: 26, pill: 270, pillFont: 23, middle: 862 };
  const nameWidth = name ? Math.min(name.length * s.font * 0.58, s.font * 9) : 0;
  const width = s.pad * 2 + (logo ? s.button + s.gap : 0) + (name ? nameWidth + s.gap : 0) + s.button + s.gap + s.pill;
  const x = portrait ? (size.w - width) / 2 : size.w - width - 44;
  let cursor = x + s.pad;
  const logoX = cursor;
  if (logo) cursor += s.button + s.gap;
  const nameX = cursor;
  if (name) cursor += nameWidth + s.gap;
  const likeX = cursor;
  cursor += s.button + s.gap;
  const thumb = s.button * 0.52;
  return (
    <g fontFamily={FONT} fontWeight="700">
      <rect x={x} y={s.middle - s.height / 2} width={width} height={s.height} rx={s.height / 2} fill="#0f0f0f" opacity={0.9} stroke="#ffffff" strokeOpacity={0.14} strokeWidth={2} />
      {logo ? <image href={logo} x={logoX} y={s.middle - s.button / 2} width={s.button} height={s.button} clipPath="circle()" preserveAspectRatio="xMidYMid slice" /> : null}
      {name ? <text x={nameX} y={s.middle + s.font * 0.35} fontSize={s.font} fill="#f1f1f1" textLength={nameWidth} lengthAdjust="spacingAndGlyphs">{name}</text> : null}
      <circle cx={likeX + s.button / 2} cy={s.middle} r={s.button / 2} fill="#272727" stroke={liked ? '#3ea6ff' : 'none'} strokeWidth={3} />
      <path d={SUBSCRIBE_THUMB} transform={`translate(${likeX + (s.button - thumb) / 2} ${s.middle - thumb / 2}) scale(${thumb / 24})`} fill={liked ? '#3ea6ff' : '#ffffff'} />
      <rect x={cursor} y={s.middle - s.button / 2} width={s.pill} height={s.button} rx={s.button / 2} fill={subscribed ? '#3a3a3a' : '#ff0033'} />
      <text x={cursor + s.pill / 2} y={s.middle + s.pillFont * 0.35} fontSize={s.pillFont} fill="white" textAnchor="middle" letterSpacing={1.5}>{subscribed ? 'SUBSCRIBED' : 'SUBSCRIBE'}</text>
    </g>
  );
}

/** CS2's notices in their corner, or ClipHub's centred feed under the vertical HUD. */
function Notices({ count, player, centered, size }: { count: number; player: string; centered: boolean; size: Size }): ReactNode {
  const width = 430;
  const x = centered ? (size.w - width) / 2 : size.w - width - 18;
  const top = size.h > size.w ? 300 : 64;
  return (
    <g fontFamily={FONT} fontWeight="700" fontSize="26">
      {Array.from({ length: count }, (_, i) => (
        <g key={i} transform={`translate(${x} ${top + i * 72})`}>
          <rect width={width} height={60} rx={8} fill="#0d1722" opacity={0.9} stroke="#e5484d" strokeWidth={3} />
          <text x={20} y={39} fill="#5d9cec">{player.slice(0, 12)}</text>
          <path d="M200 26H250L254 34H200Z" fill="#e8eef2" />
          <text x={width - 20} y={39} textAnchor="end" fill="#f4b873">{`rival ${i + 1}`}</text>
        </g>
      ))}
    </g>
  );
}

function Label({ text, x, y, size, fill = 'white', anchor = 'middle', opacity = 1 }: {
  text: string; x: number; y: number; size: number; fill?: string; anchor?: 'middle' | 'end' | 'start'; opacity?: number;
}): ReactNode {
  return <text x={x} y={y + size * 0.8} fontSize={size} fill={fill} textAnchor={anchor} fontFamily={FONT} fontWeight="800" opacity={opacity}
    stroke="black" strokeOpacity={0.9} strokeWidth={6} paintOrder="stroke" style={SHADOW}>{text}</text>;
}

/** A text on a box, as drawtext with boxborderw draws it. */
function Boxed({ text, y, size, box, border, fill = 'white', opacity, frameWidth }: {
  text: string; y: number; size: number; box: string; border: number; fill?: string; opacity: number; frameWidth: number;
}): ReactNode {
  const width = text.length * size * 0.58 + border * 2;
  return (
    <g opacity={opacity}>
      <rect x={(frameWidth - width) / 2} y={y - border} width={width} height={size + border * 2} fill={box} />
      <text x={frameWidth / 2} y={y + size * 0.8} fontSize={size} fill={fill} textAnchor="middle" fontFamily={FONT} fontWeight="800">{text}</text>
    </g>
  );
}

/** A win probability plate, its text ending or starting at x as the render's w/2±24 does. */
function Plate({ text, x, y, size, box, fill, anchor }: { text: string; x: number; y: number; size: number; box: string; fill: string; anchor: 'start' | 'end' }): ReactNode {
  const width = text.length * size * 0.58;
  const left = anchor === 'end' ? x - width : x;
  return (
    <g>
      <rect x={left - 14} y={y - 14} width={width + 28} height={size + 28} fill={box} />
      <text x={x} y={y + size * 0.8} fontSize={size} fill={fill} textAnchor={anchor} fontFamily={FONT} fontWeight="800">{text}</text>
    </g>
  );
}

/** The kill counter, badge, title, opening and closing cards, win probability and stats card. */
function Texts({ edit, frame, size, player, map }: { edit: EditConfig; frame: PreviewFrame; size: Size; player: string; map: string }): ReactNode {
  const { w, h } = size;
  const portrait = h > w;
  const text = previewTexts(edit.overlayLanguage, prettyMapName(map).toUpperCase() || 'MIRAGE');
  const title = text.title;
  const titleSize = fitFontSize(title, { base: portrait ? 72 : 58, safeWidth: w - 120 });
  const intro = edit.introText?.trim() || title;
  const outro = edit.outroText?.trim() || 'ClipHub';
  return (
    <>
      {frame.hook > 0 ? <Label text={title} x={w / 2} y={150} size={titleSize} opacity={frame.hook} /> : null}
      {frame.intro > 0 ? <Boxed text={intro} y={h / 3} size={fitFontSize(intro, { base: portrait ? 72 : 58, safeWidth: w - 120 })} box="rgb(0 0 0 / 0.55)" border={24} opacity={frame.intro} frameWidth={w} /> : null}
      {frame.winProbability > 0 ? (
        <g opacity={frame.winProbability}>
          <Label text={text.winProbability} x={w / 2} y={h * 0.24} size={portrait ? 30 : 24} />
          <Plate text={text.ctPlate} x={w / 2 - 24} y={h * 0.24 + (portrait ? 44 : 38)} size={portrait ? 46 : 36} box="rgb(93 121 174 / 0.92)" fill="white" anchor="end" />
          <Plate text={text.tPlate} x={w / 2 + 24} y={h * 0.24 + (portrait ? 44 : 38)} size={portrait ? 46 : 36} box="rgb(222 155 53 / 0.94)" fill="black" anchor="start" />
        </g>
      ) : null}
      {frame.counter ? <Label text={frame.counter.value} x={w / 2} y={h * 0.62} size={frame.counter.size} opacity={frame.counter.opacity} /> : null}
      {frame.badge ? <Boxed text={frame.badge.label} y={h * 0.7} size={60} box="rgb(255 200 61 / 0.94)" border={18} fill="black" opacity={frame.badge.opacity} frameWidth={w} /> : null}
      {frame.stats > 0 ? (
        <g opacity={frame.stats}>
          <rect width={w} height={h} fill="black" opacity={0.58} />
          <Label text={text.matchStats} x={w / 2} y={h * 0.3} size={36} />
          <Label text={player} x={w / 2} y={h * 0.35} size={fitFontSize(player, { base: 80, safeWidth: w - 140 })} />
          {text.statLines.map((line, i) => (
            <Label key={line} text={line} x={w / 2} y={h * 0.35 + 130 + i * 86} size={fitFontSize(line, { base: 50, safeWidth: w - 140 })} />
          ))}
        </g>
      ) : null}
      {frame.outro > 0 ? <Boxed text={outro} y={h * 0.7} size={portrait ? 36 : 44} box="rgb(0 0 0 / 0.42)" border={16} opacity={frame.outro} frameWidth={w} /> : null}
    </>
  );
}
