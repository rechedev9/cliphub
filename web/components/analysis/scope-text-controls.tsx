'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { listLibraryDemos, listLibrarySeries } from '@/lib/api/library';
import type { AnalysisScope } from '@/lib/analysis/scope';
import { tacticChoices, type TacticOption } from '@/lib/analysis/scope-tactics';
import { libraryTexts, toggledText, type LibraryTexts } from '@/lib/analysis/scope-texts';
import { CAPS_LABEL } from '@/components/analysis/scope-field-style';
import { Toggle } from '@/components/ui/toggle';

type Choice = { readonly value: string; readonly label: string };

const EMPTY: LibraryTexts = { tags: [], events: [] };
// One read per window: the tags and events change only on import or edit, and the menu opens often.
let cached: Promise<LibraryTexts> | null = null;

function readTexts(): Promise<LibraryTexts> {
  cached ??= Promise.all([listLibraryDemos(), listLibrarySeries()])
    .then(([demos, series]) => libraryTexts({ demos, series }))
    .catch(() => {
      cached = null;
      return EMPTY;
    });
  return cached;
}

function useLibraryTexts(): LibraryTexts | null {
  const [texts, setTexts] = useState<LibraryTexts | null>(null);
  useEffect(() => {
    let active = true;
    void readTexts().then((next) => {
      if (active) setTexts(next);
    });
    return () => {
      active = false;
    };
  }, []);
  return texts;
}

/** A labelled set of toggles, one per choice; a picked value the library no longer has still shows so it can be taken out. */
function TextChoices(props: { label: string; empty: string; value: readonly string[]; choices: readonly Choice[]; onChange: (next: string[]) => void }): ReactNode {
  const { label, empty, value, onChange } = props;
  const missing = value.filter((entry) => !props.choices.some((choice) => choice.value === entry)).map((entry) => ({ value: entry, label: entry }));
  const choices = [...props.choices, ...missing];
  return (
    <div role="group" aria-label={label} className="flex w-full flex-col gap-1">
      <span className={CAPS_LABEL}>{label}</span>
      {choices.length === 0 ? (
        <p className="text-body-sm text-fg-3">{empty}</p>
      ) : (
        <div className="flex flex-wrap gap-1">
          {choices.map((choice) => (
            <Toggle
              key={choice.value}
              variant="outline"
              size="sm"
              className="h-7 max-w-full px-2 text-body-sm"
              pressed={value.includes(choice.value)}
              onPressedChange={() => onChange(toggledText(value, choice.value))}
            >
              <span className="truncate">{choice.label}</span>
            </Toggle>
          ))}
        </div>
      )}
    </div>
  );
}

const asChoices = (values: readonly string[]): Choice[] => values.map((value) => ({ value, label: value }));

type Props = { scope: AnalysisScope; onChange: (patch: Partial<AnalysisScope>) => void };

export function MatchTagChoices({ scope, onChange, exclude }: Props & { exclude: boolean }): ReactNode {
  const texts = useLibraryTexts();
  const field = exclude ? 'excludeMatchTags' : 'matchTags';
  return (
    <TextChoices
      label={exclude ? 'Sin la etiqueta' : 'Con la etiqueta'}
      empty={texts === null ? 'Cargando etiquetas…' : 'Ninguna demo tiene etiquetas. Ponlas en Partidas.'}
      value={scope[field]}
      choices={asChoices(texts?.tags ?? [])}
      onChange={(next) => onChange(exclude ? { excludeMatchTags: next } : { matchTags: next })}
    />
  );
}

export function EventChoices({ scope, onChange }: Props): ReactNode {
  const texts = useLibraryTexts();
  return (
    <TextChoices
      label="Torneo"
      empty={texts === null ? 'Cargando torneos…' : 'Ninguna serie tiene torneo. Ponlo en la serie de la partida.'}
      value={scope.events}
      choices={asChoices(texts?.events ?? [])}
      onChange={(events) => onChange({ events })}
    />
  );
}

/** The Libreta plays of the scope's map and side whose rounds are left out. */
export function ExcludeTacticChoices({ scope, tactics, onChange }: Props & { tactics: readonly TacticOption[] }): ReactNode {
  const named = new Map(tactics.map((option) => [option.value, option.label]));
  const choices = tacticChoices(tactics, scope).filter((option) => option.value !== scope.tactic);
  const picked = scope.excludeTactics.filter((id) => !choices.some((option) => option.value === id)).map((id) => ({ value: id, label: named.get(id) ?? 'Jugada de la Libreta' }));
  return (
    <TextChoices
      label="Sin las rondas de"
      empty="Sin jugadas de la Libreta en este mapa y lado."
      value={scope.excludeTactics}
      choices={[...choices, ...picked]}
      onChange={(excludeTactics) => onChange({ excludeTactics })}
    />
  );
}
