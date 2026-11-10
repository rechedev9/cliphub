'use client';

import { useId, type ReactNode } from 'react';
import {
  KIND_LABEL,
  LIBRARY_VIEWS,
  MATCH_KINDS,
  SORTS,
  SORT_LABEL,
  type LibraryListView,
  type LibraryViewMode,
  type MatchKind,
} from '@/lib/library/partida-groups';
import { Segmented } from '@/components/ui/segmented';
import { Switch } from '@/components/ui/switch';

const ALL_KINDS = 'todas';
type KindChoice = MatchKind | typeof ALL_KINDS;

const KIND_OPTIONS: readonly { value: KindChoice; label: string }[] = [
  { value: ALL_KINDS, label: 'Todas' },
  ...MATCH_KINDS.map((kind) => ({ value: kind, label: KIND_LABEL[kind] })),
];

const SORT_OPTIONS = SORTS.map((sort) => ({ value: sort, label: SORT_LABEL[sort] }));

const VIEW_LABEL: Record<LibraryViewMode, string> = { series: 'Por series', tabla: 'Tabla' };
const VIEW_OPTIONS = LIBRARY_VIEWS.map((mode) => ({ value: mode, label: VIEW_LABEL[mode] }));

/** Above the list: how it is shown, what it is sorted by, which kind of partida and whether it needs a POV. */
export function LibraryListControls({ view, onChange }: { view: LibraryListView; onChange: (next: LibraryListView) => void }): ReactNode {
  const povId = useId();
  const voiceId = useId();
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
      <Segmented label="Vista" value={view.mode} options={VIEW_OPTIONS} onChange={(mode) => onChange({ ...view, mode })} />
      {view.mode === 'series' ? (
        <Segmented label="Ordenar por" value={view.sort} options={SORT_OPTIONS} onChange={(sort) => onChange({ ...view, sort })} />
      ) : null}
      <Segmented
        label="Tipo de partida"
        value={view.kind ?? ALL_KINDS}
        options={KIND_OPTIONS}
        onChange={(choice) => onChange({ ...view, kind: choice === ALL_KINDS ? null : choice })}
      />
      <span className="flex items-center gap-2">
        <Switch aria-labelledby={povId} checked={view.pov} onCheckedChange={(pov) => onChange({ ...view, pov })} />
        <span id={povId} className="text-body-sm text-fg-2">Con POV</span>
      </span>
      <span className="flex items-center gap-2">
        <Switch aria-labelledby={voiceId} checked={view.voice} onCheckedChange={(voice) => onChange({ ...view, voice })} />
        <span id={voiceId} className="text-body-sm text-fg-2">Con voz</span>
      </span>
    </div>
  );
}
