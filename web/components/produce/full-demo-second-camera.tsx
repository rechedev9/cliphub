'use client';

import { useId, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { FULL_DEMO_CAMERA_DEFAULT, FULL_DEMO_CAMERA_MAX, withFullDemoCameraMax, withFullDemoCameraRemoved, withFullDemoSecondCamera, type FullDemoDocument, type FullDemoOptions } from '@/lib/full-demo-plan';
import { FULL_DEMO_CAMERA_LABEL, FULL_DEMO_CAMERA_NOTE } from '@/lib/produce/full-demo-brief';
import { cameraEmptyReason, cameraOmittedText, fullDemoCameraListed, fullDemoCameraMoments } from '@/lib/produce/full-demo-camera';
import { FullDemoFold, FullDemoToggle } from './full-demo-fields';

export type FullDemoSecondCameraProps = {
  options: FullDemoOptions;
  document: FullDemoDocument | null;
  omitted: readonly string[];
  preparing: boolean;
  onChange: (value: FullDemoOptions) => void;
  onPrepare: () => void;
};

export function FullDemoSecondCamera({ options, document, omitted, preparing, onChange, onPrepare }: FullDemoSecondCameraProps): ReactNode {
  const noteId = useId();
  const camera = options.second_camera;
  const enabled = camera?.enabled === true;
  const max = camera?.max_windows || FULL_DEMO_CAMERA_DEFAULT;
  const removed = camera?.removed ?? [];
  const shown = fullDemoCameraListed(document, options);
  const moments = shown ? fullDemoCameraMoments(shown) : [];
  const listed = new Set(moments.map((moment) => moment.id));
  const gone = removed.filter((id) => !listed.has(id));
  const pending = shown !== null && (shown.options.second_camera?.removed ?? []).join(',') !== removed.join(',');
  const prepare = (label: string): ReactNode => <Button type="button" variant="secondary" className="min-h-11 self-start" onClick={onPrepare}>{label}</Button>;
  return <FullDemoFold title={FULL_DEMO_CAMERA_LABEL} value={enabled ? `Sí, hasta ${max}` : 'No'}>
    <div className="grid min-w-0">
      <FullDemoToggle label={FULL_DEMO_CAMERA_LABEL} value={enabled} describedBy={noteId} onChange={(next) => onChange(withFullDemoSecondCamera(options, next))} />
      <p id={noteId} className="pl-7 text-meta text-fg-3">{FULL_DEMO_CAMERA_NOTE}</p>
    </div>
    {enabled ? <>
      <div className="flex min-w-0 items-center gap-3 text-body-sm text-fg-1">
        <span id={`${noteId}-max`}>Máximo de momentos</span>
        <div role="group" aria-labelledby={`${noteId}-max`} className="flex items-center gap-1">
          <Button type="button" variant="secondary" size="sm" className="min-h-11 min-w-11" aria-label="Menos momentos" disabled={max <= 1} onClick={() => onChange(withFullDemoCameraMax(options, max - 1))}>−</Button>
          <output className="min-w-8 text-center font-semibold" aria-live="polite">{max}</output>
          <Button type="button" variant="secondary" size="sm" className="min-h-11 min-w-11" aria-label="Más momentos" disabled={max >= FULL_DEMO_CAMERA_MAX} onClick={() => onChange(withFullDemoCameraMax(options, max + 1))}>+</Button>
        </div>
      </div>
      {preparing ? <p role="status" className="text-body-sm text-fg-2">Buscando los momentos de la partida…</p> : null}
      {!preparing && shown === null ? <>
        <p className="text-body-sm text-fg-2">{document === null ? 'Aún no hay momentos elegidos. Búscalos para quitar los que no quieras antes de grabar.' : 'Los momentos cambian con estos ajustes. Búscalos de nuevo para verlos.'}</p>
        {prepare('Elegir momentos')}
      </> : null}
      {!preparing && shown ? <>
        {moments.length === 0 ? <p className="text-body-sm text-fg-2">{cameraEmptyReason(shown)}</p> : <ul className="flex min-w-0 flex-col divide-y divide-border-subtle">
          {moments.map((moment) => {
            const off = removed.includes(moment.id);
            return <li key={moment.id} className="flex min-w-0 items-center gap-3 py-1">
              <div className="min-w-0 flex-1">
                <p className={`truncate text-body-sm ${off ? 'text-fg-3 line-through' : 'text-fg-1'}`}>Ronda {moment.round} · {moment.enemy} · {moment.time}</p>
                {omitted.includes(moment.id) ? <p className="text-meta text-warning">{cameraOmittedText(moment.id)}</p> : null}
              </div>
              <Button type="button" variant="secondary" size="sm" className="min-h-11" onClick={() => onChange(withFullDemoCameraRemoved(options, { id: moment.id, removed: !off }))}>{off ? 'Restaurar' : 'Quitar'}</Button>
            </li>;
          })}
        </ul>}
        {gone.length > 0 ? <p className="text-body-sm text-fg-2">
          {gone.length === 1 ? 'Has quitado 1 momento.' : `Has quitado ${gone.length} momentos.`}{' '}
          <button type="button" className="min-h-11 text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-ring" onClick={() => onChange(withFullDemoCameraRemoved(options, { id: null, removed: false }))}>Restaurar todos</button>
        </p> : null}
        {pending ? <>
          <p className="text-body-sm text-fg-2">Has cambiado la selección. Actualiza los momentos para aplicarla.</p>
          {prepare('Actualizar momentos')}
        </> : null}
      </> : null}
    </> : null}
  </FullDemoFold>;
}
