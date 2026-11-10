'use client';

import type { ReactNode } from 'react';
import { Check, CircleDollarSign } from 'lucide-react';
import { buyIcon } from '@/lib/analysis/icons';
import { equipText } from '@/lib/analysis/scope-chips';
import { equipActive } from '@/lib/analysis/scope-replay-filters';
import { HALF_BUY, SCOPE_BUYS, parseScopeBuy, type AnalysisScope, type ScopeBuy } from '@/lib/analysis/scope';
import { buyLabel } from '@/lib/tactical-labels';
import { FIELD, FIELD_SET } from '@/components/analysis/scope-field-style';
import { EquipRangeControl } from '@/components/analysis/scope-replay-controls';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

type BuyChoice = { readonly value: ScopeBuy; readonly label: string };

/** Every buy a person can ask for; `half` stands for a semi or a force. */
export const BUY_CHOICES: readonly BuyChoice[] = [
  { value: '', label: 'Todas las compras' },
  ...SCOPE_BUYS.map((buy): BuyChoice => ({ value: buy, label: buyLabel(buy) })),
  { value: HALF_BUY, label: 'Media (semi o force)' },
];

function choiceLabel(buy: ScopeBuy): string {
  return BUY_CHOICES.find((choice) => choice.value === buy)?.label ?? 'Todas las compras';
}

function BuyGlyph({ buy }: { buy: ScopeBuy }): ReactNode {
  const known = SCOPE_BUYS.find((entry) => entry === buy);
  const icon = known === undefined ? null : buyIcon(known);
  return icon === null ? <span aria-hidden className="size-3" /> : <EquipmentIcon name={icon} height={12} className="text-fg-2" />;
}

export type EconomyFieldProps = {
  scope: AnalysisScope;
  onChange: (patch: Partial<AnalysisScope>) => void;
  /** The rival's economy instead of the team's own. */
  rival: boolean;
  /** Name in the popover header, such as the team; empty reads "propia". */
  name?: string;
  showBuy: boolean;
  showEquip: boolean;
};

/** One side's economy as a popover: the buy types, and the custom equipment range per player. */
export function EconomyField({ scope, onChange, rival, name, showBuy, showEquip }: EconomyFieldProps): ReactNode {
  const buy = rival ? scope.opponentBuy : scope.buy;
  const range = rival ? scope.opponentEquip : scope.equip;
  const ranged = showEquip && equipActive(range);
  const set = buy !== '' || ranged;
  const label = ranged ? equipText(range) : choiceLabel(buy);
  const title = rival ? 'Economía del rival' : 'Economía propia';
  const header = rival ? 'ECONOMÍA DEL RIVAL' : `ECONOMÍA ${name === undefined || name === '' ? 'PROPIA' : name.toUpperCase()}`;
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`${title}: ${label}`}
          className={cn(FIELD, 'max-w-44 min-w-28 justify-start font-medium normal-case tracking-normal', set && FIELD_SET)}
        >
          {rival ? <span aria-hidden className="text-meta font-semibold text-fg-3">RIV</span> : <CircleDollarSign aria-hidden className="size-3.5 text-fg-3" />}
          <span className="truncate">{label}</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-0">
        <h3 className="truncate border-b border-border px-2 py-1.5 text-meta font-medium tracking-wider text-fg-3">{header}</h3>
        {showBuy ? (
          <ul aria-label={title} className="p-1">
            {BUY_CHOICES.map((choice) => (
              <li key={choice.value}>
                <button
                  type="button"
                  aria-pressed={buy === choice.value}
                  onClick={() => onChange(rival ? { opponentBuy: parseScopeBuy(choice.value) } : { buy: parseScopeBuy(choice.value) })}
                  className={cn(
                    'flex min-h-8 w-full items-center gap-2 rounded-sm px-2 text-left text-body-sm text-popover-foreground hover:bg-surface-5',
                    buy === choice.value && 'text-primary',
                  )}
                >
                  <BuyGlyph buy={choice.value} />
                  <span className="flex-1">{choice.label}</span>
                  {buy === choice.value ? <Check aria-hidden className="size-4" /> : null}
                </button>
              </li>
            ))}
          </ul>
        ) : null}
        {showEquip ? (
          <div className="flex flex-col gap-1 border-t border-dashed border-border-strong px-2 py-2">
            <span className="text-meta font-medium tracking-wider text-fg-3 uppercase">Rango a medida</span>
            <EquipRangeControl rival={rival} scope={scope} onChange={onChange} />
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}
