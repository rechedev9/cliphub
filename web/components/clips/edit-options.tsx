'use client';

import { Film, Gift, Headphones, ImageIcon, ListOrdered, Monitor, PanelTop, Sparkles, Type, Zap } from 'lucide-react';
import {
  AFFILIATE_FAMILY_CATALOG,
  BOOKEND_TEXT_MAX_LENGTH,
  DEFAULT_KEYDROP_CODE,
  KEYDROP_CODE_RE,
  affiliateDisplayLabel,
  affiliateFamilyLabel,
  isAffiliateStyle,
  stylesForFamily,
  type EditConfig,
} from '@/lib/api/types';
import { selectAffiliateFamily, selectAffiliateOff, selectAffiliateStyle } from '@/lib/affiliate-banner';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

/** Show the live character counter only once the input is getting close to the limit. */
const COUNTER_THRESHOLD = BOOKEND_TEXT_MAX_LENGTH - 20;

export type EditOptionsProps = {
  value: EditConfig;
  onChange: (next: EditConfig) => void;
  disabled?: boolean;
  /** Recap / comms / native HUD live on /full-demo, not the Shorts constructor. */
  showFullDemoDelivery?: boolean;
};

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

/** Kill-effect, transition, and bookend controls. Aspect lives in CreateReelBar. */
export function EditOptions({
  value,
  onChange,
  disabled = false,
  showFullDemoDelivery = false,
}: EditOptionsProps) {
  return (
    <div className={cn('grid gap-4 md:grid-cols-[1fr_1fr]', disabled && 'opacity-60')}>
      {showFullDemoDelivery ? (
        <OptionBlock label="ENTREGA OPCIONAL" className="md:col-span-2">
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
                className="w-36 shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2"
              >
                COMMS <span className="text-stream-text">· {Math.round((value.voiceVolume ?? 0.85) * 100)}%</span>
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
                className="h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-stream disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>
          ) : null}
        </OptionBlock>
      ) : null}

      <OptionBlock label="EFECTO DE KILL">
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

      <OptionBlock label="TRANSICIONES">
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
              <Sparkles className="size-4" />
              {item.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </OptionBlock>

      <OptionBlock label="TEXTO AUTOMÁTICO" className="md:col-span-2">
        <ToggleGroup
          type="multiple"
          value={[value.hookText ? 'hook-text' : '', value.killCounter ? 'kill-counter' : ''].filter(Boolean)}
          onValueChange={(items) =>
            onChange({
              ...value,
              hookText: items.includes('hook-text'),
              killCounter: items.includes('kill-counter'),
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
          <ToggleGroupItem value="kill-counter" aria-label="Contador de kills">
            <ListOrdered className="size-4" />
            Contador de kills
          </ToggleGroupItem>
        </ToggleGroup>
      </OptionBlock>

      <OptionBlock label="APERTURA Y CIERRE" className="md:col-span-2">
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
            placeholder="Título de apertura (vacío = titular generado)"
            disabled={disabled}
            onChange={(introText) => onChange({ ...value, introText })}
          />
          <BookendTextField
            label="Texto de cierre"
            value={value.outroText ?? ''}
            visible={value.outro}
            placeholder="Texto de cierre (tu handle; vacío = ClipHub)"
            disabled={disabled}
            onChange={(outroText) => onChange({ ...value, outroText })}
          />
        </div>
      </OptionBlock>

      <OptionBlock label="PORTADA" className="md:col-span-2">
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
          <ToggleGroupItem value="generated-gameplay" aria-label="Generar candidatos de gameplay">
            <ImageIcon className="size-4" />
            Candidatos de gameplay
          </ToggleGroupItem>
          <ToggleGroupItem value="no-cover" aria-label="No generar portada">
            Sin portada
          </ToggleGroupItem>
        </ToggleGroup>
      </OptionBlock>

      <OptionBlock label="BANNER AFILIADO" className="md:col-span-2">
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
              <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-wide text-muted-foreground">
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
                Se renderiza como{' '}
                <span className="font-mono text-fg-1">
                  {affiliateDisplayLabel(value.keyDropFamily ?? '', value.keyDropStyle ?? '', value.keyDropCode ?? '')}
                </span>
              </p>
            </div>
            <div className="grid max-w-sm gap-3 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-wide text-muted-foreground">
                  Entra (s)
                </span>
                <Input
                  type="number"
                  min={0}
                  step={0.1}
                  value={value.keyDropStartSeconds ?? 0}
                  disabled={disabled}
                  className="font-mono"
                  onChange={(e) =>
                    onChange({ ...value, keyDropStartSeconds: Number(e.target.value) })
                  }
                  aria-label="Segundo de entrada del banner"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-wide text-muted-foreground">
                  Sale (s)
                </span>
                <Input
                  type="number"
                  min={0.1}
                  step={0.1}
                  value={value.keyDropEndSeconds ?? 4}
                  disabled={disabled}
                  className="font-mono"
                  onChange={(e) =>
                    onChange({ ...value, keyDropEndSeconds: Number(e.target.value) })
                  }
                  aria-label="Segundo de salida del banner"
                />
              </div>
            </div>
            <p className="text-body-sm text-fg-3">
              La placa solo se ve entre esos segundos del reel (por defecto 0–4s).
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
          <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-wide text-muted-foreground">
            {label}
          </span>
          {near ? (
            <span
              className={cn(
                'font-[family-name:var(--font-mono)] text-[10px] tabular-nums text-muted-foreground',
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
      <span className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-wide text-muted-foreground">
        {label}
      </span>
      {children}
    </div>
  );
}
