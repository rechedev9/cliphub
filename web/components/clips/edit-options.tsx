'use client';

import { ChartColumn, Crosshair, Film, Gift, Headphones, ImageIcon, ListOrdered, Monitor, PanelTop, Percent, Rewind, Trophy, Type, Zap } from 'lucide-react';
import {
  AFFILIATE_FAMILY_CATALOG,
  BOOKEND_TEXT_MAX_LENGTH,
  DEFAULT_KEYDROP_CODE,
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { BrandKitOptions } from './brand-kit-options';

/** Show the live character counter only once the input is getting close to the limit. */
const COUNTER_THRESHOLD = BOOKEND_TEXT_MAX_LENGTH - 20;

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
};

/** "0,25x": the decimal comma of the Spanish UI. */
function speedLabel(speed: number): string {
  return `${String(speed).replace('.', ',')}x`;
}

const effectItems: Array<{ value: EditConfig['killEffect']; label: string }> = [
  { value: 'punch-in', label: 'Impacto' },
  { value: 'clean', label: 'Limpio' },
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

/** Kill-effect, transition, and bookend controls. Aspect is the produce format. */
export function EditOptions({
  value,
  onChange: emit,
  disabled = false,
  showFullDemoDelivery = false,
  killCounter = true,
  statsCard = true,
}: EditOptionsProps) {
  const onChange = (next: EditConfig): void => emit(exclusiveBanner(value, next));
  return (
    <div className="grid gap-4 md:grid-cols-[1fr_1fr]">
      {showFullDemoDelivery ? (
        <OptionBlock label="Entrega opcional" className="md:col-span-2">
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
                className="h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-primary disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>
          ) : null}
        </OptionBlock>
      ) : null}

      <OptionBlock label="Efecto de kill">
        <ToggleGroup
          type="single"
          value={value.killEffect}
          onValueChange={(killEffect) =>
            killEffect && onChange({ ...value, killEffect: killEffect as EditConfig['killEffect'] })
          }
          disabled={disabled}
          variant="outline"
          className="flex-wrap"
        >
          {effectItems.map((item) => (
            <ToggleGroupItem key={item.value} value={item.value} aria-label={item.label}>
              <Zap className="size-4" />
              {item.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </OptionBlock>

      <OptionBlock label="Transiciones">
        <ToggleGroup
          type="single"
          value={value.transition}
          onValueChange={(transition) =>
            transition && onChange({ ...value, transition: transition as EditConfig['transition'] })
          }
          disabled={disabled}
          variant="outline"
          className="flex-wrap"
        >
          {transitionItems.map((item) => (
            <ToggleGroupItem key={item.value} value={item.value} aria-label={item.label}>
              {item.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </OptionBlock>

      <OptionBlock label="Texto automático" className="md:col-span-2">
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

      {showFullDemoDelivery ? null : (
        <OptionBlock label="Repetición" className="md:col-span-2">
          <ToggleGroup
            type="multiple"
            value={value.victimReplay ? ['victim-replay'] : []}
            onValueChange={(items) => onChange({ ...value, victimReplay: items.includes('victim-replay') })}
            disabled={disabled}
            variant="outline"
            className="flex-wrap"
          >
            <ToggleGroupItem value="victim-replay" aria-label="Desde la víctima">
              <Rewind className="size-4" />
              Desde la víctima
            </ToggleGroupItem>
          </ToggleGroup>
          {value.victimReplay ? (
            <>
              <ToggleGroup
                type="single"
                value={String(value.victimReplaySpeed ?? DEFAULT_VICTIM_REPLAY_SPEED)}
                onValueChange={(raw) => {
                  const speed = Number(raw);
                  if (isVictimReplaySpeed(speed)) onChange({ ...value, victimReplaySpeed: speed });
                }}
                disabled={disabled}
                variant="outline"
                className="flex-wrap"
                aria-label="Velocidad de la repetición"
              >
                {VICTIM_REPLAY_SPEEDS.map((speed) => (
                  <ToggleGroupItem key={speed} value={String(speed)} aria-label={`Velocidad ${speedLabel(speed)}`}>
                    {speedLabel(speed)}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
              <p className="text-body-sm text-fg-3">
                Tras cada jugada, su última baja otra vez, vista por quien muere. A 1x va a velocidad real.
              </p>
            </>
          ) : null}
        </OptionBlock>
      )}

      {value.format === 'short-9x16' && statsCard && !showFullDemoDelivery ? (
        <OptionBlock label="Tarjeta final" className="md:col-span-2">
          <ToggleGroup
            type="multiple"
            value={value.statsCard ? ['stats-card'] : []}
            onValueChange={(items) => onChange({ ...value, statsCard: items.includes('stats-card') })}
            disabled={disabled}
            variant="outline"
            className="flex-wrap"
          >
            <ToggleGroupItem value="stats-card" aria-label="Estadísticas del jugador">
              <ChartColumn className="size-4" />
              Estadísticas del jugador
            </ToggleGroupItem>
          </ToggleGroup>
          <p className="text-body-sm text-fg-3">
            Cierra el Short con K/D, ADR, HS % y el rating de ClipHub del jugador. Si FACEIT tiene sus datos, añade su nivel, su porcentaje de victorias y su mejor mapa.
          </p>
        </OptionBlock>
      ) : null}

      {value.format === 'short-9x16' ? (
        <OptionBlock label="Kill feed" className="md:col-span-2">
          <ToggleGroup
            type="multiple"
            value={value.centeredKillfeed ? ['centered-killfeed'] : []}
            onValueChange={(items) => onChange({ ...value, centeredKillfeed: items.includes('centered-killfeed') })}
            disabled={disabled}
            variant="outline"
            className="flex-wrap"
          >
            <ToggleGroupItem value="centered-killfeed" aria-label="Kill feed centrada arriba">
              <Crosshair className="size-4" />
              Centrada arriba
            </ToggleGroupItem>
          </ToggleGroup>
          <p className="text-body-sm text-fg-3">
            ClipHub dibuja tus kills en el centro, arriba del Short, en lugar de la kill feed de CS2.
          </p>
        </OptionBlock>
      ) : null}

      <OptionBlock label="Apertura y cierre" className="md:col-span-2">
        <ToggleGroup
          type="multiple"
          value={[value.intro ? 'intro' : '', value.outro ? 'outro' : ''].filter(Boolean)}
          onValueChange={(items) =>
            onChange({ ...value, intro: items.includes('intro'), outro: items.includes('outro') })
          }
          disabled={disabled}
          variant="outline"
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

        <div className="grid gap-3 sm:grid-cols-2">
          <BookendTextField
            label="Título de apertura"
            value={value.introText ?? ''}
            visible={value.intro}
            placeholder="Ej. ACE en Mirage (si lo dejas vacío, se genera solo)"
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

      <OptionBlock label="Portada" className="md:col-span-2">
        <ToggleGroup
          type="single"
          value={value.coverStrategy}
          onValueChange={(coverStrategy) =>
            coverStrategy && onChange({ ...value, coverStrategy: coverStrategy as EditConfig['coverStrategy'] })
          }
          disabled={disabled}
          variant="outline"
          className="flex-wrap"
        >
          <ToggleGroupItem value="generated-gameplay" aria-label="Fotograma del vídeo">
            <ImageIcon className="size-4" />
            Fotograma del vídeo
          </ToggleGroupItem>
          <ToggleGroupItem value="no-cover" aria-label="No generar portada">
            Sin portada
          </ToggleGroupItem>
        </ToggleGroup>
      </OptionBlock>

      {showFullDemoDelivery ? null : (
        <OptionBlock label="Kit de marca" className="md:col-span-2">
          <BrandKitOptions value={value} onChange={onChange} disabled={disabled} />
        </OptionBlock>
      )}

      <OptionBlock label="Banner afiliado" className="md:col-span-2">
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
          className="flex-wrap"
          aria-label="Familia del banner"
        >
          {AFFILIATE_FAMILY_CATALOG.map((entry) => (
            <ToggleGroupItem key={entry.id} value={entry.id} aria-label={`Familia ${entry.label}`}>
              {entry.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
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
          className="flex-wrap"
          aria-label="Estilo del banner"
        >
          <ToggleGroupItem value="off" aria-label="Sin banner">
            {value.keyDropFamily
              ? AFFILIATE_FAMILY_CATALOG.find((entry) => entry.id === value.keyDropFamily)?.offLabel ?? 'Sin banner'
              : 'Sin banner'}
          </ToggleGroupItem>
          {stylesForFamily(value.keyDropFamily || 'KEYDROP').map((entry) => (
            <ToggleGroupItem key={entry.id} value={entry.id} aria-label={`Estilo ${entry.label}`}>
              <Gift className="size-4" />
              {entry.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        {value.keyDropStyle ? (
          <div className="flex max-w-md flex-col gap-3 pt-1">
            <div className="flex max-w-sm flex-col gap-1.5">
              <span className="text-meta text-fg-3">
                Código
              </span>
              <Input
                value={value.keyDropCode ?? ''}
                placeholder={DEFAULT_KEYDROP_CODE}
                maxLength={16}
                disabled={disabled}
                className="font-mono uppercase"
                aria-invalid={
                  Boolean(value.keyDropCode?.trim()) &&
                  !KEYDROP_CODE_RE.test((value.keyDropCode ?? '').trim())
                }
                onChange={(e) => onChange({ ...value, keyDropCode: e.target.value.toUpperCase() })}
                aria-label={`Código ${affiliateFamilyLabel(value.keyDropFamily ?? '', value.keyDropStyle)}`}
              />
              <p className="text-body-sm text-fg-3">
                En el vídeo se lee{' '}
                <span className="font-mono text-fg-1">
                  {affiliateDisplayLabel(value.keyDropFamily ?? '', value.keyDropStyle ?? '', value.keyDropCode ?? '')}
                </span>
              </p>
            </div>
            <div className="grid max-w-sm gap-3 sm:grid-cols-2">
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
      </OptionBlock>
    </div>
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
        'overflow-hidden transition-[max-height,opacity] duration-200 ease-out',
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
        />
      </div>
    </div>
  );
}

function OptionBlock({ label, className, children }: { label: string; className?: string; children: React.ReactNode }) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-2', className)}>
      <span className="text-meta text-fg-3">{label}</span>
      {children}
    </div>
  );
}
