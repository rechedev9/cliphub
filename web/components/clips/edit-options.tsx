'use client';

import { ChartColumn, ChevronRight, Crosshair, Film, Gift, Headphones, ImageIcon, ListOrdered, Monitor, PanelTop, Percent, Rewind, Spline, Trophy, Type, UserRound } from 'lucide-react';
import type { ReactNode } from 'react';
import {
  AFFILIATE_FAMILY_CATALOG,
  BOOKEND_TEXT_MAX_LENGTH,
  KEYDROP_CODE_RE,
  DEFAULT_VICTIM_REPLAY_SPEED,
  VICTIM_REPLAY_SPEEDS,
  affiliateDisplayLabel,
  affiliateFamilyLabel,
  isAffiliateStyle,
  isVictimReplaySpeed,
  stylesForFamily,
  type EditConfig,
} from '@/lib/api/types';
import { selectAffiliateFamily, selectAffiliateOff, selectAffiliateStyle } from '@/lib/affiliate-banner';
import { exclusiveBanner } from '@/lib/brand-kit';
import { OVERLAY_LANGUAGE_LABEL, OVERLAY_LANGUAGES, isOverlayLanguage } from '@/lib/overlay-language';
import { victimReplaySeconds } from '@/lib/produce/short-selection';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { BrandKitOptions } from './brand-kit-options';
import { PlayerCardOptions } from './player-card-options';
import { RadarOptions } from './radar-options';
import { VerticalHudOptions } from './vertical-hud-options';

/** Show the live character counter only once the input is getting close to the limit. */
const COUNTER_THRESHOLD = BOOKEND_TEXT_MAX_LENGTH - 20;
/** Separate chips, so a wrapped row keeps its borders instead of one broken plate. */
const CHIP_SPACING = 1.5;

export type EditOptionsProps = {
  value: EditConfig;
  onChange: (next: EditConfig) => void;
  disabled?: boolean;
  /** Recap / comms / native HUD live on /full-demo, not the Shorts constructor. */
  showFullDemoDelivery?: boolean;
  /** False hides the kill counter: a multi-POV video counts no single player's kills. */
  killCounter?: boolean;
  /** False hides the stats card: a multi-POV video has no single player to sum up. */
  statsCard?: boolean;
  /** False disables the vertical HUD: the chosen style records CS2's own HUD. */
  verticalHud?: boolean;
  /** False disables ClipHub's radar: the chosen style records CS2's own HUD and radar. */
  radar?: boolean;
  /** The job a one-player 9:16 Short's player cards read; unset hides them. */
  playerCards?: { jobId: string; blockedReason?: string | undefined };
};

/** "0,25x": the decimal comma of the Spanish UI. */
function speedLabel(speed: number): string {
  return `${String(speed).replace('.', ',')}x`;
}

const effectItems: Array<{ value: EditConfig['killEffect']; label: string; title?: string }> = [
  { value: 'punch-in', label: 'Impacto', title: 'Un zoom rápido hacia la mira en cada baja.' },
  { value: 'clean', label: 'Limpio', title: 'Sin efecto en las bajas.' },
  { value: 'velocity', label: 'Velocidad' },
  { value: 'freeze-flash', label: 'Congelado' },
  { value: 'shake', label: 'Terremoto' },
  { value: 'glitch', label: 'Glitch' },
];

const transitionItems: Array<{ value: EditConfig['transition']; label: string }> = [
  { value: 'flash', label: 'Destello' },
  { value: 'cut', label: 'Corte' },
  { value: 'whip', label: 'Barrido' },
  { value: 'dip', label: 'Fundido' },
  { value: 'glitch', label: 'Glitch' },
  { value: 'zoom-whip', label: 'Zoom-whip' },
];

/** Kill-effect, transition, and bookend controls, in folding groups. Aspect is the produce format. */
export function EditOptions({
  value,
  onChange: emit,
  disabled = false,
  showFullDemoDelivery = false,
  killCounter = true,
  statsCard = true,
  verticalHud = true,
  radar = true,
  playerCards,
}: EditOptionsProps) {
  const onChange = (next: EditConfig): void => emit(exclusiveBanner(value, next));
  const portraitShort = value.format === 'short-9x16' && !showFullDemoDelivery && !value.deathReel;
  const hudFeed = portraitShort && verticalHud && value.verticalHud === true;
  const replayExtra = Math.round(victimReplaySeconds({ victimReplay: true, victimReplaySpeed: value.victimReplaySpeed ?? DEFAULT_VICTIM_REPLAY_SPEED }));
  return (
    <div className="@container/options flex min-w-0 flex-col gap-2">
      <OptionGroup title="Efectos y transiciones" defaultOpen>
        {showFullDemoDelivery ? (
          <OptionBlock label="Entrega opcional" choice="many">
            <ToggleGroup
              type="multiple"
              value={[
                value.matchRecap ? 'match-recap' : '',
                value.voiceComms ? 'voice-comms' : '',
                value.nativeHud ? 'native-hud' : '',
              ].filter(Boolean)}
              onValueChange={(items) =>
                onChange({
                  ...value,
                  matchRecap: items.includes('match-recap'),
                  voiceComms: items.includes('voice-comms'),
                  nativeHud: items.includes('native-hud'),
                })
              }
              disabled={disabled}
              variant="outline"
              spacing={CHIP_SPACING}
              className="flex-wrap"
            >
              <ToggleGroupItem value="match-recap" aria-label="Resumen de partida">
                <Film className="size-4" />
                Resumen de partida
              </ToggleGroupItem>
              <ToggleGroupItem value="voice-comms" aria-label="Comms del equipo">
                <Headphones className="size-4" />
                Comms del equipo
              </ToggleGroupItem>
              <ToggleGroupItem value="native-hud" aria-label="HUD nativo">
                <Monitor className="size-4" />
                HUD nativo
              </ToggleGroupItem>
            </ToggleGroup>
            {value.voiceComms ? (
              <div className="flex items-center gap-4 pt-1">
                <label
                  htmlFor="voice-volume"
                  className="w-36 shrink-0 text-meta text-fg-2"
                >
                  Comms <span className="text-fg-1">· {Math.round((value.voiceVolume ?? 0.85) * 100)} %</span>
                </label>
                <input
                  id="voice-volume"
                  type="range"
                  min={0}
                  max={100}
                  step={5}
                  value={Math.round((value.voiceVolume ?? 0.85) * 100)}
                  disabled={disabled}
                  onChange={(e) => onChange({ ...value, voiceVolume: Number(e.target.value) / 100 })}
                  className={cn('h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong disabled:cursor-not-allowed disabled:opacity-50', FORMAT_TONE[value.format === 'short-9x16' ? 'short' : 'summary'].accent)}
                />
              </div>
            ) : null}
          </OptionBlock>
        ) : null}

        <OptionBlock label="Efecto de kill" choice="one" half>
          <ToggleGroup
            type="single"
            value={value.killEffect}
            onValueChange={(killEffect) =>
              killEffect && onChange({ ...value, killEffect: killEffect as EditConfig['killEffect'] })
            }
            disabled={disabled}
            variant="outline"
            spacing={CHIP_SPACING}
            className="flex-wrap"
          >
            {effectItems.map((item) => (
              <ToggleGroupItem key={item.value} value={item.value} aria-label={item.label} title={item.title} className="group/choice">
                <ChoiceDot />
                {item.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </OptionBlock>

        <OptionBlock label="Transiciones" choice="one" half>
          <ToggleGroup
            type="single"
            value={value.transition}
            onValueChange={(transition) =>
              transition && onChange({ ...value, transition: transition as EditConfig['transition'] })
            }
            disabled={disabled}
            variant="outline"
            spacing={CHIP_SPACING}
            className="flex-wrap"
          >
            {transitionItems.map((item) => (
              <ToggleGroupItem key={item.value} value={item.value} aria-label={item.label} className="group/choice">
                <ChoiceDot />
                {item.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </OptionBlock>
      </OptionGroup>

      <OptionGroup title="Textos">
        {showFullDemoDelivery ? null : (
          <OptionBlock label="Idioma de los textos del vídeo" choice="one">
            <ToggleGroup
              type="single"
              value={value.overlayLanguage ?? 'es'}
              onValueChange={(language) => {
                if (isOverlayLanguage(language)) onChange({ ...value, overlayLanguage: language });
              }}
              disabled={disabled}
              variant="outline"
              spacing={CHIP_SPACING}
              className="flex-wrap"
              aria-label="Idioma de los textos del vídeo"
            >
              {OVERLAY_LANGUAGES.map((language) => (
                <ToggleGroupItem key={language} value={language} aria-label={OVERLAY_LANGUAGE_LABEL[language]} className="group/choice">
                  <ChoiceDot />
                  {OVERLAY_LANGUAGE_LABEL[language]}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <p className="text-body-sm text-fg-3">
              El título automático, las insignias, la probabilidad de victoria, las tarjetas y el marcador vertical. Los textos que escribes tú salen tal cual.
            </p>
          </OptionBlock>
        )}

        <OptionBlock label="Texto automático" choice="many">
          <ToggleGroup
            type="multiple"
            value={[value.hookText ? 'hook-text' : '', value.killCounter ? 'kill-counter' : '', value.playBadges ? 'play-badges' : '', value.winProbability ? 'win-probability' : ''].filter(Boolean)}
            onValueChange={(items) =>
              onChange({
                ...value,
                hookText: items.includes('hook-text'),
                killCounter: items.includes('kill-counter'),
                playBadges: items.includes('play-badges'),
                winProbability: items.includes('win-probability'),
              })
            }
            disabled={disabled}
            variant="outline"
            spacing={CHIP_SPACING}
            className="flex-wrap"
          >
            <ToggleGroupItem value="hook-text" aria-label="Título automático">
              <Type className="size-4" />
              Título automático
            </ToggleGroupItem>
            {killCounter ? (
              <ToggleGroupItem value="kill-counter" aria-label="Contador de kills">
                <ListOrdered className="size-4" />
                Contador de kills
              </ToggleGroupItem>
            ) : null}
            <ToggleGroupItem value="play-badges" aria-label="Insignias de jugada">
              <Trophy className="size-4" />
              Insignias de jugada
            </ToggleGroupItem>
            <ToggleGroupItem value="win-probability" aria-label="Probabilidad de victoria">
              <Percent className="size-4" />
              Probabilidad de victoria
            </ToggleGroupItem>
          </ToggleGroup>
          {value.playBadges ? (
            <p className="text-body-sm text-fg-3">
              Tras la última baja de cada jugada, una insignia nombra el clutch (1vX, ganado si se sabe), el ace o la multikill.
            </p>
          ) : null}
          {value.winProbability ? (
            <p className="text-body-sm text-fg-3">
              Tras el plantado, el % de victoria de cada equipo mientras queden 5 s o más de bomba y 2 o más vivos por bando. Las partidas analizadas antes de esta versión no lo muestran hasta reanalizarlas.
            </p>
          ) : null}
        </OptionBlock>

        <OptionBlock label="Apertura y cierre" choice="many">
          <ToggleGroup
            type="multiple"
            value={[value.intro ? 'intro' : '', value.outro ? 'outro' : ''].filter(Boolean)}
            onValueChange={(items) =>
              onChange({ ...value, intro: items.includes('intro'), outro: items.includes('outro') })
            }
            disabled={disabled}
            variant="outline"
            spacing={CHIP_SPACING}
            className="flex-wrap"
          >
            <ToggleGroupItem value="intro" aria-label="Apertura">
              <PanelTop className="size-4" />
              Apertura
            </ToggleGroupItem>
            <ToggleGroupItem value="outro" aria-label="Cierre">
              <PanelTop className="size-4 rotate-180" />
              Cierre
            </ToggleGroupItem>
          </ToggleGroup>

          <div className="grid min-w-0 gap-3 @[34rem]/options:grid-cols-2">
            <BookendTextField
              label="Título de apertura"
              value={value.introText ?? ''}
              visible={value.intro}
              placeholder="Ej. ACE en Mirage (vacío: se genera solo)"
              disabled={disabled}
              onChange={(introText) => onChange({ ...value, introText })}
            />
            <BookendTextField
              label="Texto de cierre"
              value={value.outroText ?? ''}
              visible={value.outro}
              placeholder="Ej. @tunick (por defecto: ClipHub)"
              disabled={disabled}
              onChange={(outroText) => onChange({ ...value, outroText })}
            />
          </div>
        </OptionBlock>
      </OptionGroup>

      <OptionGroup title="Sobre la jugada">
        {showFullDemoDelivery ? null : (
          <OptionBlock label="Repetición">
            <ToggleGroup
              type="multiple"
              value={value.victimReplay ? ['victim-replay'] : []}
              onValueChange={(items) => onChange({ ...value, victimReplay: items.includes('victim-replay') })}
              disabled={disabled}
              variant="outline"
              spacing={CHIP_SPACING}
              className="flex-wrap"
            >
              <ToggleGroupItem value="victim-replay" aria-label="Desde la víctima">
                <Rewind className="size-4" />
                Desde la víctima
              </ToggleGroupItem>
            </ToggleGroup>
            {value.victimReplay ? (
              <>
                <span className="text-meta text-fg-3">Velocidad · elige una</span>
                <ToggleGroup
                  type="single"
                  value={String(value.victimReplaySpeed ?? DEFAULT_VICTIM_REPLAY_SPEED)}
                  onValueChange={(raw) => {
                    const speed = Number(raw);
                    if (isVictimReplaySpeed(speed)) onChange({ ...value, victimReplaySpeed: speed });
                  }}
                  disabled={disabled}
                  variant="outline"
                  spacing={CHIP_SPACING}
                  className="flex-wrap"
                  aria-label="Velocidad de la repetición"
                >
                  {VICTIM_REPLAY_SPEEDS.map((speed) => (
                    <ToggleGroupItem key={speed} value={String(speed)} aria-label={`Velocidad ${speedLabel(speed)}`} className="group/choice">
                      <ChoiceDot />
                      {speedLabel(speed)}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
                <p className="text-body-sm text-fg-3">
                  Tras cada jugada, su última baja otra vez, vista por quien muere. A 1x va a velocidad real. Alarga el vídeo unos {replayExtra} s por jugada.
                </p>
              </>
            ) : null}
          </OptionBlock>
        )}

        {showFullDemoDelivery || value.matchRecap ? null : (
          <OptionBlock label="Sobre el juego" choice="many">
            <ToggleGroup
              type="multiple"
              value={[value.playerLabels ? 'player-labels' : '', value.grenadeTrails ? 'grenade-trails' : ''].filter(Boolean)}
              onValueChange={(items) =>
                onChange({ ...value, playerLabels: items.includes('player-labels'), grenadeTrails: items.includes('grenade-trails') })
              }
              disabled={disabled}
              variant="outline"
              spacing={CHIP_SPACING}
              className="flex-wrap"
            >
              <ToggleGroupItem value="player-labels" aria-label="Nombres y vida">
                <UserRound className="size-4" />
                Nombres y vida
              </ToggleGroupItem>
              <ToggleGroupItem value="grenade-trails" aria-label="Estelas de granadas">
                <Spline className="size-4" />
                Estelas de granadas
              </ToggleGroupItem>
            </ToggleGroup>
            {value.playerLabels || value.grenadeTrails ? (
              <p className="text-body-sm text-fg-3">
                Se leen de la demo y se dibujan sobre la jugada: nombre y vida de tus compañeros y de los rivales que CS2 marca como vistos o que dañas, y la trayectoria de las granadas de tu equipo. Se ocultan con la mira de zoom o al morir y se ven a través de las paredes.
              </p>
            ) : null}
          </OptionBlock>
        )}

        {showFullDemoDelivery || value.matchRecap ? null : (
          <OptionBlock label="Radar">
            <RadarOptions value={value} onChange={onChange} disabled={disabled} allowed={radar && !value.nativeHud} />
          </OptionBlock>
        )}

        {portraitShort ? (
          <OptionBlock label="Marcador">
            <VerticalHudOptions value={value} onChange={onChange} disabled={disabled} allowed={verticalHud} />
          </OptionBlock>
        ) : null}

        {value.format === 'short-9x16' ? (
          <OptionBlock label="Kill feed">
            <ToggleGroup
              type="multiple"
              value={value.centeredKillfeed || hudFeed ? ['centered-killfeed'] : []}
              onValueChange={(items) => onChange({ ...value, centeredKillfeed: items.includes('centered-killfeed') })}
              disabled={disabled || hudFeed}
              variant="outline"
              spacing={CHIP_SPACING}
              className="flex-wrap"
            >
              <ToggleGroupItem value="centered-killfeed" aria-label="Kill feed centrada arriba">
                <Crosshair className="size-4" />
                Centrada arriba
              </ToggleGroupItem>
            </ToggleGroup>
            <p className="text-body-sm text-fg-3">
              {hudFeed
                ? 'El HUD vertical la incluye, debajo del marcador.'
                : 'ClipHub dibuja tus kills en el centro, arriba del Short, en lugar de la kill feed de CS2.'}
            </p>
          </OptionBlock>
        ) : null}
      </OptionGroup>

      <OptionGroup title="Final, portada y marca">
        {value.format === 'short-9x16' && playerCards && !showFullDemoDelivery && !value.matchRecap ? (
          <OptionBlock label="Cartas de jugador">
            <PlayerCardOptions value={value} onChange={onChange} disabled={disabled} jobId={playerCards.jobId} blockedReason={playerCards.blockedReason} />
          </OptionBlock>
        ) : null}

        {value.format === 'short-9x16' && statsCard && !showFullDemoDelivery ? (
          <OptionBlock label="Tarjeta final">
            <ToggleGroup
              type="multiple"
              value={value.statsCard && !value.endCard ? ['stats-card'] : []}
              onValueChange={(items) => onChange({ ...value, statsCard: items.includes('stats-card') })}
              disabled={disabled || value.endCard !== undefined}
              variant="outline"
              spacing={CHIP_SPACING}
              className="flex-wrap"
            >
              <ToggleGroupItem value="stats-card" aria-label="Estadísticas del jugador">
                <ChartColumn className="size-4" />
                Estadísticas del jugador
              </ToggleGroupItem>
            </ToggleGroup>
            <p className="text-body-sm text-fg-3">
              {value.endCard
                ? 'La carta de jugador cierra el Short en su lugar.'
                : 'Cierra el Short con K/D, ADR, HS % y el rating de ClipHub del jugador. Si FACEIT tiene sus datos, añade su nivel, su porcentaje de victorias y su mejor mapa.'}
            </p>
          </OptionBlock>
        ) : null}

        <OptionBlock label="Portada" choice="one">
          <ToggleGroup
            type="single"
            value={value.coverStrategy}
            onValueChange={(coverStrategy) =>
              coverStrategy && onChange({ ...value, coverStrategy: coverStrategy as EditConfig['coverStrategy'] })
            }
            disabled={disabled}
            variant="outline"
            spacing={CHIP_SPACING}
            className="flex-wrap"
          >
            <ToggleGroupItem value="generated-gameplay" aria-label="Fotograma del vídeo" className="group/choice">
              <ChoiceDot />
              <ImageIcon className="size-4" />
              Fotograma del vídeo
            </ToggleGroupItem>
            <ToggleGroupItem value="no-cover" aria-label="No generar portada" className="group/choice">
              <ChoiceDot />
              Sin portada
            </ToggleGroupItem>
          </ToggleGroup>
          {value.coverCard && value.coverStrategy !== 'no-cover' && value.format === 'short-9x16' ? (
            <p className="text-body-sm text-fg-3">La carta de jugador va encima del fotograma.</p>
          ) : null}
        </OptionBlock>

        {showFullDemoDelivery ? null : (
          <OptionBlock label="Kit de marca">
            <BrandKitOptions value={value} onChange={onChange} disabled={disabled} />
          </OptionBlock>
        )}

        <OptionBlock label="Banner afiliado">
          <AffiliateBannerOptions value={value} onChange={onChange} disabled={disabled} />
        </OptionBlock>
      </OptionGroup>
    </div>
  );
}

/** The affiliate plate: family, design, the user's own code and when it shows. */
function AffiliateBannerOptions({ value, onChange, disabled }: { value: EditConfig; onChange: (next: EditConfig) => void; disabled: boolean }): ReactNode {
  const styles = stylesForFamily(value.keyDropFamily || 'KEYDROP');
  const code = (value.keyDropCode ?? '').trim();
  const codeValid = KEYDROP_CODE_RE.test(code);
  const familyLabel = affiliateFamilyLabel(value.keyDropFamily ?? '', value.keyDropStyle ?? '') || 'KeyDrop';
  return (
    <>
      <span className="text-meta text-fg-3">Web de afiliados · elige una</span>
      <ToggleGroup
        type="single"
        value={value.keyDropFamily || ''}
        onValueChange={(next) => {
          if (next !== 'KEYDROP' && next !== 'CSGOSKINS') return;
          const selected = selectAffiliateFamily(
            { family: value.keyDropFamily ?? '', style: value.keyDropStyle ?? '' },
            next,
          );
          onChange({
            ...value,
            keyDropFamily: selected.family,
            keyDropStyle: selected.style,
            keyDropStartSeconds: value.keyDropStartSeconds ?? 0,
            keyDropEndSeconds: value.keyDropEndSeconds ?? 4,
          });
        }}
        disabled={disabled}
        variant="outline"
        spacing={CHIP_SPACING}
        className="flex-wrap"
        aria-label="Familia del banner"
      >
        {AFFILIATE_FAMILY_CATALOG.map((entry) => (
          <ToggleGroupItem key={entry.id} value={entry.id} aria-label={`Familia ${entry.label}`} className="group/choice">
            <ChoiceDot />
            {entry.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <span className="text-meta text-fg-3">Diseño de la placa · elige uno</span>
      <ToggleGroup
        type="single"
        value={value.keyDropStyle || 'off'}
        onValueChange={(next) => {
          if (!next) return;
          if (next === 'off') {
            const off = selectAffiliateOff();
            onChange({ ...value, keyDropFamily: off.family, keyDropStyle: off.style, keyDropCode: value.keyDropCode });
            return;
          }
          const selected = selectAffiliateStyle(value.keyDropFamily ?? '', next);
          if (!selected.style || !isAffiliateStyle(selected.family, selected.style)) return;
          onChange({
            ...value,
            keyDropFamily: selected.family,
            keyDropStyle: selected.style,
            keyDropStartSeconds: value.keyDropStartSeconds ?? 0,
            keyDropEndSeconds: value.keyDropEndSeconds ?? 4,
          });
        }}
        disabled={disabled}
        variant="outline"
        spacing={CHIP_SPACING}
        className="flex-wrap"
        aria-label="Estilo del banner"
      >
        <ToggleGroupItem value="off" aria-label="Sin banner" className="group/choice">
          <ChoiceDot />
          {value.keyDropFamily
            ? AFFILIATE_FAMILY_CATALOG.find((entry) => entry.id === value.keyDropFamily)?.offLabel ?? 'Sin banner'
            : 'Sin banner'}
        </ToggleGroupItem>
        {styles.map((entry) => (
          <ToggleGroupItem key={entry.id} value={entry.id} aria-label={`Estilo ${entry.label}`} title={entry.subtitle} className="group/choice">
            <ChoiceDot />
            <Gift className="size-4" />
            {entry.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <p className="text-body-sm text-fg-3">
        {styles.map((entry) => entry.label).join(', ')} son diseños de la placa de {familyLabel}. Sobre ella se lee tu propio código de afiliado.
      </p>
      {value.keyDropStyle ? (
        <div className="flex min-w-0 flex-col gap-3 pt-1">
          <div className="flex min-w-0 flex-col gap-1.5">
            <span className="text-meta text-fg-3">
              Tu código de afiliado
            </span>
            <Input
              value={value.keyDropCode ?? ''}
              placeholder="Ej. TUCODIGO"
              maxLength={16}
              disabled={disabled}
              className="font-mono uppercase"
              aria-invalid={!codeValid}
              onChange={(e) => onChange({ ...value, keyDropCode: e.target.value.toUpperCase() })}
              aria-label={`Código ${affiliateFamilyLabel(value.keyDropFamily ?? '', value.keyDropStyle)}`}
            />
            {codeValid ? (
              <p className="text-body-sm text-fg-3">
                En el vídeo se lee{' '}
                <span className="font-mono text-fg-1">
                  {affiliateDisplayLabel(value.keyDropFamily ?? '', value.keyDropStyle, code)}
                </span>
              </p>
            ) : (
              <p role="alert" className="text-body-sm text-destructive">
                {code === ''
                  ? `Escribe tu código de ${familyLabel}: sin él no se puede crear el vídeo con este banner.`
                  : 'Usa de 1 a 16 letras, números, guiones o guiones bajos, sin espacios.'}
              </p>
            )}
          </div>
          <div className="grid min-w-0 gap-3 @[20rem]/options:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <span className="text-meta text-fg-3">
                Entra (s)
              </span>
              <Input
                type="number"
                min={0}
                step={0.1}
                value={value.keyDropStartSeconds ?? 0}
                disabled={disabled}
                onChange={(e) =>
                  onChange({ ...value, keyDropStartSeconds: Number(e.target.value) })
                }
                aria-label="Segundo de entrada del banner"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <span className="text-meta text-fg-3">
                Sale (s)
              </span>
              <Input
                type="number"
                min={0.1}
                step={0.1}
                value={value.keyDropEndSeconds ?? 4}
                disabled={disabled}
                onChange={(e) =>
                  onChange({ ...value, keyDropEndSeconds: Number(e.target.value) })
                }
                aria-label="Segundo de salida del banner"
              />
            </div>
          </div>
          <p className="text-body-sm text-fg-3">
            La placa solo se ve entre esos segundos del vídeo (por defecto, de 0 a 4 s).
          </p>
        </div>
      ) : null}
    </>
  );
}

function BookendTextField({
  label,
  value,
  visible,
  placeholder,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  visible: boolean;
  placeholder: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const near = value.length >= COUNTER_THRESHOLD;
  return (
    <div
      className={cn(
        'min-w-0 overflow-hidden transition-[max-height,opacity] duration-200 ease-out',
        visible ? 'max-h-24 opacity-100 visible' : 'max-h-0 opacity-0 invisible',
      )}
    >
      <div className="flex flex-col gap-1.5 pt-1">
        <div className="flex items-center justify-between gap-2">
          <span className="text-meta text-fg-3">
            {label}
          </span>
          {near ? (
            <span
              className={cn(
                // [font-size:…] not `text-meta`: tailwind-merge files an unknown
                // `text-*` under text-COLOUR, so the step would lose to text-fg-3.
                '[font-size:var(--text-meta)] tabular-nums text-fg-3',
                value.length >= BOOKEND_TEXT_MAX_LENGTH && 'text-destructive',
              )}
            >
              {value.length}/{BOOKEND_TEXT_MAX_LENGTH}
            </span>
          ) : null}
        </div>
        <Input
          value={value}
          placeholder={placeholder}
          maxLength={BOOKEND_TEXT_MAX_LENGTH}
          disabled={disabled || !visible}
          tabIndex={visible ? 0 : -1}
          onChange={(e) => onChange(e.target.value)}
          aria-label={label}
          title={placeholder}
          className="w-full"
        />
      </div>
    </div>
  );
}

/** One folding group of the panel; nested in "Textos y gráficos", so only the first starts open. */
function OptionGroup({ title, defaultOpen = false, children }: { title: string; defaultOpen?: boolean; children: ReactNode }) {
  return (
    <details open={defaultOpen} className="group/optgroup min-w-0 rounded-xl border border-border-subtle">
      <summary className="flex min-h-10 cursor-pointer list-none items-center justify-between gap-2 rounded-xl px-3 text-body-sm font-semibold text-fg-1 focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
        {title}
        <ChevronRight aria-hidden className="size-4 text-fg-2 transition-transform duration-(--dur-fast) group-open/optgroup:rotate-90" />
      </summary>
      <div className="grid min-w-0 gap-4 border-t border-border-subtle p-3 @[34rem]/options:grid-cols-2">{children}</div>
    </details>
  );
}

const CHOICE_HINT = { one: 'elige uno', many: 'puedes activar varios' } as const;

function OptionBlock({ label, choice, half = false, children }: { label: string; choice?: keyof typeof CHOICE_HINT; half?: boolean; children: ReactNode }) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-2', !half && '@[34rem]/options:col-span-2')}>
      <span className="text-meta text-fg-3">
        {label}
        {choice ? <span className="text-fg-4"> · {CHOICE_HINT[choice]}</span> : null}
      </span>
      {children}
    </div>
  );
}

/** A radio mark: tells a pick-one chip from the switches that can all be on. */
function ChoiceDot() {
  return (
    <span aria-hidden className="grid size-3.5 shrink-0 place-items-center rounded-full border-[1.5px] border-current">
      <span className="size-1.5 rounded-full bg-current opacity-0 group-data-[state=on]/choice:opacity-100" />
    </span>
  );
}
