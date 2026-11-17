'use client';

import { useEffect, useId, useState, type ReactNode } from 'react';
import { CUSTOM_HUD_THEMES, customHudTheme } from '@/lib/custom-hud';
import {
  MAX_HUD_RADIUS, MAX_HUD_TEAM_NAME, MAX_HUD_THEME_NAME, createHudTheme, deleteHudTheme, duplicateStyle, hudColor, hudStyleOf, listHudThemes, sameHudStyle, updateHudTheme,
  type HudOptions, type HudStyle, type HudTeamNames, type UserHudTheme,
} from '@/lib/hud-themes';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { FullDemoChoice, FullDemoPart } from './full-demo-fields';

const USER = 'user:';

/** The design select with the user's own themes, and the editor of the chosen one. */
export function HudThemePicker({ base, style, onSelect }: {
  base: string; style?: HudStyle; onSelect: (base: string, style?: HudStyle) => void;
}): ReactNode {
  const [themes, setThemes] = useState<UserHudTheme[]>([]);
  const [max, setMax] = useState(10);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [message, setMessage] = useState<{ tone: 'error' | 'info'; text: string } | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    listHudThemes(controller.signal).then((list) => { setThemes(list.themes); setMax(list.max); }).catch((failure: unknown) => {
      if (!controller.signal.aborted) setMessage({ tone: 'error', text: failure instanceof Error ? failure.message : 'No se pudieron leer tus temas.' });
    });
    return () => controller.abort();
  }, []);

  const matched = style ? themes.find((theme) => theme.base === base && sameHudStyle(hudStyleOf(theme), style)) : undefined;
  const editing = themes.find((theme) => theme.id === (editingId ?? matched?.id));
  const value = editing && style ? `${USER}${editing.id}` : base;
  const dirty = editing !== undefined && style !== undefined && !sameHudStyle(hudStyleOf(editing), style);
  const options = [
    ...CUSTOM_HUD_THEMES.map((theme) => ({ value: theme.id, label: theme.name })),
    ...themes.map((theme) => ({ value: `${USER}${theme.id}`, label: `${theme.name} (tema propio)` })),
  ];

  function choose(next: string): void {
    setConfirmDelete(false); setMessage(null);
    const theme = themes.find((entry) => `${USER}${entry.id}` === next);
    setEditingId(theme?.id ?? null);
    if (theme) onSelect(theme.base, hudStyleOf(theme));
    else onSelect(next);
  }

  async function run(action: () => Promise<void>): Promise<void> {
    setBusy(true); setMessage(null);
    try { await action(); } catch (failure) { setMessage({ tone: 'error', text: failure instanceof Error ? failure.message : 'No se pudo guardar el tema.' }); }
    finally { setBusy(false); }
  }

  const duplicate = (): Promise<void> => run(async () => {
    const created = await createHudTheme(base, duplicateStyle(base, style));
    setThemes((list) => [...list, created]);
    setEditingId(created.id);
    onSelect(created.base, hudStyleOf(created));
    setMessage({ tone: 'info', text: 'Tema creado. Cambia sus colores y guárdalo.' });
  });

  const save = (): Promise<void> => run(async () => {
    if (!editing || !style) return;
    const saved = await updateHudTheme(editing.id, style);
    setThemes((list) => list.map((theme) => theme.id === saved.id ? saved : theme));
    setMessage({ tone: 'info', text: 'Tema guardado.' });
  });

  const remove = (): Promise<void> => run(async () => {
    if (!editing) return;
    await deleteHudTheme(editing.id);
    setThemes((list) => list.filter((theme) => theme.id !== editing.id));
    setEditingId(null); setConfirmDelete(false);
    onSelect(editing.base);
  });

  const full = themes.length >= max;
  const baseName = customHudTheme(base)?.name ?? base;
  return <div className="grid min-w-0 gap-2">
    <FullDemoChoice label="Diseño" value={value} options={options} onChange={choose} />
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      <Button type="button" variant="secondary" disabled={busy || full} onClick={() => void duplicate()}>Duplicar y editar</Button>
      <span className="text-body-sm text-fg-3">{full ? `Tienes ${max} temas, el máximo. Borra uno para crear otro.` : `Tus temas: ${themes.length} de ${max}.`}</span>
    </div>
    {editing && style ? <ThemeEditor key={editing.id} baseName={baseName} style={style} onChange={(next) => { setEditingId(editing.id); onSelect(editing.base, next); }} /> : null}
    {style && !editing ? <div className="flex min-w-0 flex-wrap items-center gap-2">
      <p className="text-body-sm text-fg-2">Este plan usa el tema {style.name}, que ya no está entre tus temas guardados.</p>
      <Button type="button" variant="secondary" disabled={busy} onClick={() => onSelect(base)}>Volver a {baseName}</Button>
    </div> : null}
    {editing && style ? <div className="flex min-w-0 flex-wrap gap-2">
      <Button type="button" disabled={busy || !dirty} onClick={() => void save()}>{dirty ? 'Guardar cambios' : 'Guardado'}</Button>
      {confirmDelete
        ? <><Button type="button" variant="destructive" disabled={busy} onClick={() => void remove()}>Sí, borrar tema</Button>
          <Button type="button" variant="secondary" disabled={busy} onClick={() => setConfirmDelete(false)}>No borrar</Button></>
        : <Button type="button" variant="secondary" disabled={busy} onClick={() => setConfirmDelete(true)}>Borrar tema</Button>}
    </div> : null}
    {message ? <p role={message.tone === 'error' ? 'alert' : 'status'} className={`text-body-sm ${message.tone === 'error' ? 'text-destructive' : 'text-fg-2'}`}>{message.text}</p> : null}
  </div>;
}

function ThemeEditor({ baseName, style, onChange }: { baseName: string; style: HudStyle; onChange: (style: HudStyle) => void }): ReactNode {
  const id = useId();
  const radius = style.radius;
  const colors: readonly { key: 'ct' | 't' | 'background'; label: string }[] = [
    { key: 'ct', label: 'Color CT' }, { key: 't', label: 'Color T' }, { key: 'background', label: 'Fondo de las placas' },
  ];
  function withRadius(next: number | undefined): HudStyle {
    const { radius: _radius, ...rest } = style;
    return next === undefined ? rest : { ...rest, radius: next };
  }
  return <FullDemoPart title="Tu tema" note={`Copia de ${baseName}. Solo cambia el dibujo del HUD: no hace falta volver a grabar.`}>
    <div className="grid min-w-0 gap-3">
      <div className="space-y-1.5">
        <label htmlFor={`${id}-name`} className="text-body-sm text-fg-2">Nombre</label>
        <Input id={`${id}-name`} value={style.name} maxLength={MAX_HUD_THEME_NAME}
          onChange={(event) => { const name = event.target.value; if (name.trim() !== '') onChange({ ...style, name }); }} />
      </div>
      <div className="grid min-w-0 grid-cols-3 gap-2">
        {colors.map((color) => <label key={color.key} className="flex min-w-0 flex-col gap-1.5 text-body-sm text-fg-2">
          {color.label}
          <input type="color" value={`#${style[color.key].toLowerCase()}`} onChange={(event) => onChange({ ...style, [color.key]: hudColor(event.target.value) })}
            className="h-10 w-full min-w-0 cursor-pointer rounded-md border border-border-subtle bg-transparent" />
        </label>)}
      </div>
      <label className="flex min-w-0 cursor-pointer items-center gap-3 text-body-sm text-fg-1">
        <input type="checkbox" className="size-4 accent-format-full" checked={radius !== undefined} onChange={(event) => onChange(withRadius(event.target.checked ? 6 : undefined))} />
        Esquinas a medida
      </label>
      {radius !== undefined ? <div className="flex min-w-0 items-center gap-3">
        <label htmlFor={`${id}-radius`} className="w-28 shrink-0 text-meta text-fg-2">Radio <span className="text-fg-1">· {radius} px</span></label>
        <input id={`${id}-radius`} type="range" min={0} max={MAX_HUD_RADIUS} step={1} value={radius} aria-valuetext={`${radius} px`}
          className="h-1 w-full min-w-0 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-format-full" onChange={(event) => onChange(withRadius(Number(event.target.value)))} />
      </div> : <p className="text-body-sm text-fg-3">Sin marcar, las placas mantienen la forma de {baseName}.</p>}
    </div>
  </FullDemoPart>;
}

const SWITCHES: readonly { key: keyof HudOptions; label: string; detail: string }[] = [
  { key: 'compact_header', label: 'Cabecera compacta', detail: 'Quita las fichas de los diez jugadores de arriba y deja el marcador.' },
  { key: 'hide_loadout', label: 'Ocultar inventario', detail: 'Sin el arma ni la munición del jugador grabado.' },
  { key: 'round_win_banner', label: 'Mensaje de ronda ganada', detail: 'Al cerrar cada ronda, el equipo que la gana bajo el marcador.' },
  { key: 'win_probability', label: 'Probabilidad de victoria', detail: 'Tras el plantado, el % de victoria de cada equipo bajo el marcador, con 5 s o más de bomba y 2 o más vivos por bando.' },
];

/** The composition switches of overlays.hud_options. */
export function HudCompositionSwitches({ options, className, onOptions }: {
  options?: HudOptions; className?: string; onOptions: (options: HudOptions) => void;
}): ReactNode {
  const id = useId();
  return <FullDemoPart title="Composición" note="Solo cambian el dibujo del HUD, sin volver a grabar." className={className}>
      {SWITCHES.map((entry) => <label key={entry.key} className="flex min-w-0 cursor-pointer items-start gap-3 text-body-sm">
        <input type="checkbox" checked={options?.[entry.key] === true} aria-describedby={`${id}-${entry.key}`} className="mt-0.5 size-4 shrink-0 accent-format-full"
          onChange={(event) => onOptions({ ...options, [entry.key]: event.target.checked })} />
        <span className="min-w-0"><span className="block font-semibold text-fg-1">{entry.label}</span>
          <span id={`${id}-${entry.key}`} className="mt-0.5 block text-fg-2">{entry.detail}</span></span>
      </label>)}
  </FullDemoPart>;
}

/** The team names of the scoreboard. */
export function HudTeamNameFields({ names, onNames }: { names?: HudTeamNames; onNames: (names: HudTeamNames) => void }): ReactNode {
  return <FullDemoPart title="Equipos en el marcador" note="Vienen de la biblioteca cuando la partida tiene equipos. Vacío, el marcador usa el nombre de la demo o CT y T.">
      <div className="grid min-w-0 gap-2 @min-[22rem]/hud:grid-cols-2">
        <TeamNameField label="Empieza de CT" value={names?.ct_start ?? ''} onChange={(ct_start) => onNames({ ...names, ct_start })} />
        <TeamNameField label="Empieza de T" value={names?.t_start ?? ''} onChange={(t_start) => onNames({ ...names, t_start })} />
      </div>
  </FullDemoPart>;
}

/** Typing keeps inner spaces; the plan gets the trimmed name, as Go validates it. */
function TeamNameField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }): ReactNode {
  const id = useId();
  const [draft, setDraft] = useState(value);
  useEffect(() => { setDraft((current) => current.trim() === value ? current : value); }, [value]);
  return <div className="min-w-0 space-y-1.5">
    <label htmlFor={id} className="text-body-sm text-fg-2">{label}</label>
    <Input id={id} value={draft} maxLength={MAX_HUD_TEAM_NAME} placeholder="Nombre del equipo"
      onChange={(event) => { setDraft(event.target.value); onChange(event.target.value.trim()); }} onBlur={() => setDraft(value)} />
  </div>;
}
