'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import dynamic from 'next/dynamic';
import { RefreshCw } from 'lucide-react';
import { MapGeometryError } from '@/lib/api/map-geometry';
import type { MapGeometry } from '@/lib/api/map-geometry';
import type { MapScene } from '@/lib/api/map-scene';
import { prettyMapName } from '@/lib/format';
import { docNames } from '@/lib/replay/centre-round';
import { playBox } from '@/lib/replay/three/scene-math';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import type { ReplayCentreProps, ReplayCentreView } from './centre';
import type { CameraMode } from './three/camera-rig';
import { ReplayChip } from './replay-chip';
import { loadMapGeometry, loadMapScene, loadNavGeometry } from './three/geometry-cache';
import { loadRadarImage } from './radar/map-art';

const ThreeScene = dynamic(() => import('./three/three-scene').then((module) => module.ThreeScene), {
  ssr: false,
  loading: () => <Skeleton role="status" aria-label="Cargando la vista 3D" className="h-full w-full rounded-none" />,
});

const CAMERA_OPTIONS: readonly { value: CameraMode; label: string }[] = [
  { value: 'orbit', label: 'Órbita' },
  { value: 'top', label: 'Cenital' },
  { value: 'player', label: 'Jugador' },
];

const CAMERA_KEYS: Record<string, CameraMode> = { '1': 'orbit', '2': 'top', '3': 'player' };

const SLIDER_CLASS = 'h-1 w-32 min-w-0 cursor-pointer appearance-none rounded-full bg-border-strong accent-primary';

type RealState =
  | { map: string; state: 'loading'; share: number }
  | { map: string; state: 'ready'; scene: MapScene }
  | { map: string; state: 'failed' };

type GeometryState =
  | { map: string; state: 'loading' }
  | { map: string; state: 'ready'; geometry: MapGeometry; nav: MapGeometry | null }
  | { map: string; state: 'failed'; code: string | undefined };

/** What went wrong in the user's words, and whether trying again can help. */
function failureText(code: string | undefined, map: string): { text: string; retry: boolean } {
  const name = prettyMapName(map);
  switch (code) {
    case 'cs2_not_found':
      return { text: 'La vista 3D lee el mapa de tu instalación de CS2 y no la encontramos en este equipo.', retry: true };
    case 'map_not_found':
      return { text: `Tu instalación de CS2 no tiene ${name}. Descárgalo en CS2 para verlo en 3D.`, retry: true };
    case 'map_has_no_nav':
      return { text: `${name} no trae los datos que la vista 3D necesita para dibujar el suelo.`, retry: false };
    case 'map_geometry_unreadable':
      return { text: `No se pudo leer la geometría de ${name}. Puede que CS2 haya cambiado su formato.`, retry: false };
    default:
      return { text: 'No se pudo cargar el mapa en 3D. Studio no respondió.', retry: true };
  }
}

function webglAvailable(): boolean {
  try {
    const canvas = document.createElement('canvas');
    const context = canvas.getContext('webgl2');
    context?.getExtension('WEBGL_lose_context')?.loseContext();
    return context !== null;
  } catch {
    return false;
  }
}

/** Typing in a field must not switch cameras. */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT') return true;
  return target.tagName === 'INPUT' && target.getAttribute('type') !== 'range' && target.getAttribute('type') !== 'checkbox';
}

/**
 * The 3D centre: the map from the user's CS2 install with the round's players
 * and grenades on it, for analysis only. Three.js loads with this view.
 */
function ThreeCentre({ mode: roundMode, rounds, clock, tickOf, layers, onSelectView, followSlot, onFollowSlot }: ReplayCentreProps): ReactNode {
  const focused = rounds.find((round) => round.focused) ?? rounds.at(-1);
  const map = focused?.doc.demo.map ?? '';
  const [webgl, setWebgl] = useState<boolean | null>(null);
  const [crashed, setCrashed] = useState(false);
  const [load, setLoad] = useState<GeometryState | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [camera, setCamera] = useState<CameraMode>('orbit');
  const [cut, setCut] = useState<{ map: string; z: number } | null>(null);
  const [auto, setAuto] = useState(true);
  const [radarOn, setRadarOn] = useState(false);
  const [radarImage, setRadarImage] = useState<{ map: string; image: HTMLImageElement } | null>(null);
  const [real, setReal] = useState<RealState | null>(null);
  const [realOn, setRealOn] = useState(true);

  useEffect(() => setWebgl(webglAvailable()), []);

  // The opening cut is chosen once per map from the round open at that moment.

  useEffect(() => {
    if (map === '') return;
    let live = true;
    setLoad({ map, state: 'loading' });
    Promise.all([loadMapGeometry(map), loadNavGeometry(map)]).then(
      ([loaded, nav]) => {
        if (!live) return;
        setLoad({ map, state: 'ready', geometry: loaded, nav });
      },
      (error: unknown) => {
        if (live) setLoad({ map, state: 'failed', code: error instanceof MapGeometryError ? error.code : undefined });
      },
    );
    return () => {
      live = false;
    };
  }, [map, attempt]);

  useEffect(() => {
    if (map === '') return;
    let live = true;
    setReal({ map, state: 'loading', share: 0 });
    loadMapScene(map, (share) => {
      if (live) setReal({ map, state: 'loading', share });
    }).then(
      (scene) => {
        if (live) setReal({ map, state: 'ready', scene });
      },
      (error: unknown) => {
        console.error('3D: the real map could not be read', error);
        if (live) setReal({ map, state: 'failed' });
      },
    );
    return () => {
      live = false;
    };
  }, [map, attempt]);

  useEffect(() => {
    if (map === '') return;
    let live = true;
    void loadRadarImage(map, 'upper').then((image) => {
      if (live) setRadarImage(image === null ? null : { map, image });
    });
    return () => {
      live = false;
    };
  }, [map]);
  const calibration = focused?.doc.geometry.calibration ?? null;
  const radar = useMemo(
    () => (radarImage === null || radarImage.map !== map || calibration === null ? null : { image: radarImage.image, calibration }),
    [calibration, map, radarImage],
  );

  const geometry = load?.map === map && load.state === 'ready' ? load.geometry : null;
  const realScene = real?.map === map && real.state === 'ready' ? real.scene : null;
  const realShare = real?.map === map && real.state === 'loading' ? real.share : null;
  const nav = load?.map === map && load.state === 'ready' ? load.nav : null;
  const focusedFrames = focused?.frames;
  const focusedBounds = focused?.doc.geometry.bounds;
  const box = useMemo(() => {
    if (geometry === null) return null;
    const [minX, minY, minZ] = geometry.bounds.min;
    const [maxX, maxY, maxZ] = geometry.bounds.max;
    return playBox({ min: { x: minX, y: minY, z: minZ }, max: { x: maxX, y: maxY, z: maxZ } }, focusedBounds, focusedFrames);
  }, [geometry, focusedBounds, focusedFrames]);
  const range = geometry === null ? null : { min: Math.floor(geometry.bounds.min[2]), max: Math.ceil(geometry.bounds.max[2]) };
  const cutZ = cut?.map === map ? cut.z : null;

  const names = useMemo(() => (focused === undefined ? new Map<number, string>() : docNames(focused.doc)), [focused]);
  const followed = followSlot === null ? null : (names.get(followSlot) ?? null);

  const selectCamera = useCallback(
    (next: CameraMode) => {
      setCamera(next);
      if (next !== 'player') {
        onFollowSlot(null);
        return;
      }
      if (followSlot === null && focused !== undefined) onFollowSlot(focused.round.players[0]?.slot ?? null);
    },
    [focused, followSlot, onFollowSlot],
  );

  useEffect(() => {
    if (followSlot !== null) setCamera('player');
  }, [followSlot]);

  useEffect(() => {
    const listener = (event: KeyboardEvent): void => {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || isTyping(event.target)) return;
      const next = CAMERA_KEYS[event.key];
      if (next === undefined) return;
      event.preventDefault();
      selectCamera(next);
    };
    window.addEventListener('keydown', listener);
    return () => window.removeEventListener('keydown', listener);
  }, [selectCamera]);

  const pick = useCallback((slot: number) => onFollowSlot(slot), [onFollowSlot]);
  const fail = useCallback(() => setCrashed(true), []);

  const label = roundMode === 'multi'
    ? `Vista 3D de ${rounds.length} rondas superpuestas en ${prettyMapName(map)}`
    : `Vista 3D de la ronda ${focused?.round.number ?? ''} en ${prettyMapName(map)}`;

  let body: ReactNode;
  if (webgl === false || crashed) {
    body = (
      <Notice
        text="Este equipo no puede dibujar en 3D: la tarjeta gráfica o su controlador no ofrece WebGL 2."
        onBack={() => onSelectView('2d')}
      />
    );
  } else if (load?.map === map && load.state === 'failed') {
    const failure = failureText(load.code, map);
    body = <Notice text={failure.text} onBack={() => onSelectView('2d')} onRetry={failure.retry ? () => setAttempt((value) => value + 1) : undefined} />;
  } else if (geometry === null || box === null || webgl === null) {
    body = <Skeleton role="status" aria-label={`Cargando ${prettyMapName(map)} en 3D`} className="h-full w-full rounded-none" />;
  } else {
    body = (
      <ThreeScene
        geometry={geometry}
        nav={nav}
        radarOn={radarOn && radar !== null}
        scene={realScene}
        realOn={realOn}
        box={box}
        cut={cutZ}
        auto={auto}
        onAutoChange={setAuto}
        mode={camera}
        follow={followSlot}
        rounds={rounds}
        tickOf={tickOf}
        layers={layers}
        clock={clock}
        onPick={pick}
        onFailure={fail}
        label={label}
        radar={radar}
      />
    );
  }

  return (
    <div className="absolute inset-0 overflow-hidden bg-surface-0">
      {body}
      <div className="pointer-events-none absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] flex-wrap items-center gap-2">
        <div className="pointer-events-auto">
          <Segmented label="Cámara" value={camera} options={CAMERA_OPTIONS} onChange={selectCamera} />
        </div>
        {camera === 'orbit' ? (
          <div className="pointer-events-auto">
            <ReplayChip pressed={auto} onClick={() => setAuto((value) => !value)} title="La cámara sigue a los jugadores vivos hasta que la mueves">
              Seguir la acción
            </ReplayChip>
          </div>
        ) : null}
        {realScene === null ? null : (
          <div className="pointer-events-auto">
            <ReplayChip pressed={realOn} onClick={() => setRealOn((value) => !value)} title="Dibuja el mapa con sus texturas y la luz de CS2, o el esquema por alturas">
              Mapa real
            </ReplayChip>
          </div>
        )}
        {realShare === null ? null : (
          <span role="status" className="rounded-md bg-surface-1/95 px-2 py-1 text-meta text-fg-2 tabular-nums">
            Cargando el mapa real {Math.round(realShare * 100)} %
          </span>
        )}
        {radar === null ? null : (
          <div className="pointer-events-auto">
            <ReplayChip pressed={radarOn} onClick={() => setRadarOn((value) => !value)} title="Pinta el radar oficial del juego sobre los suelos">
              Textura de radar
            </ReplayChip>
          </div>
        )}
        {range === null || camera === 'top' ? null : (
          <div className="pointer-events-auto flex h-9 items-center gap-2 rounded-md border border-border-strong bg-surface-1/95 px-2.5 text-meta text-fg-2">
            <ReplayChip pressed={cutZ === null} onClick={() => setCut(null)} title="Quita los techos y corta las paredes a poca altura sobre cada suelo">
              Techos auto
            </ReplayChip>
            <input
              type="range"
              aria-label="Cortar techos"
              min={range.min}
              max={range.max}
              step={8}
              value={Math.min(range.max, Math.max(range.min, Math.round(cutZ ?? range.max)))}
              aria-valuetext={cutZ === null ? 'Automático' : `Por encima de ${Math.round(cutZ)} unidades`}
              className={SLIDER_CLASS}
              onChange={(event) => setCut({ map, z: Number(event.target.value) })}
            />
          </div>
        )}
        {camera === 'player' && followed !== null ? (
          <span className="rounded-md bg-surface-1/95 px-2 py-1 text-meta text-fg-1">Ves lo que ve {followed}</span>
        ) : null}
      </div>
      {geometry?.source === 'nav' ? (
        <p className="pointer-events-none absolute top-16 left-2 max-w-80 rounded-md bg-surface-0/85 px-2 py-1 text-meta text-fg-2">
          Solo se ve el suelo: las paredes de este mapa no se pudieron leer.
        </p>
      ) : null}
    </div>
  );
}

function Notice({ text, onBack, onRetry }: { text: string; onBack: () => void; onRetry?: () => void }): ReactNode {
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-3 bg-surface-1 px-6 text-center">
      <p className="max-w-md text-body-sm text-fg-2">{text}</p>
      <div className="flex flex-wrap justify-center gap-2">
        <Button type="button" size="sm" variant="outline" onClick={onBack}>
          Volver a 2D
        </Button>
        {onRetry === undefined ? null : (
          <Button type="button" size="sm" variant="ghost" onClick={onRetry}>
            <RefreshCw aria-hidden />
            Reintentar
          </Button>
        )}
      </div>
    </div>
  );
}

/** The 3D centre view; put it after `RADAR_2D_VIEW` in `centreViews`. */
export const THREE_D_VIEW: ReplayCentreView = {
  id: '3d',
  label: '3D',
  followsPlayers: true,
  render: (props) => <ThreeCentre {...props} />,
};
