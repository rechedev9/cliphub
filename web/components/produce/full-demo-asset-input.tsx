'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { fullDemoProvenance, fullDemoSourceError, uploadFullDemoAsset, type FullDemoAssetRef, type FullDemoProvenance } from '@/lib/full-demo-plan';

const EMPTY_PROVENANCE: FullDemoProvenance = { title: '', creator: '', source_url: '', permission: '', attribution: '' };

export function FullDemoAssetInput({ label, accept, open, onUploaded, onBusyChange }: {
  label: string; accept: string;
  /**
   * Expanded while its option still needs a file, so enabling the option shows
   * the picker right away. React only writes the attribute when this changes:
   * the user can still collapse it, and it folds once a file is added.
   */
  open?: boolean;
  onUploaded: (ref: FullDemoAssetRef) => void; onBusyChange: (busy: boolean) => void;
}): ReactNode {
  const id = useId();
  const fileInput = useRef<HTMLInputElement>(null);
  const rights = useRef<HTMLDetailsElement>(null);
  const sourceInput = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [provenance, setProvenance] = useState<FullDemoProvenance>(EMPTY_PROVENANCE);
  const [sourceError, setSourceError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  async function upload(): Promise<void> {
    if (!file) return;
    const invalidSource = fullDemoSourceError(provenance.source_url);
    if (invalidSource) {
      setSourceError(invalidSource);
      if (rights.current) rights.current.open = true;
      sourceInput.current?.focus();
      return;
    }
    const controller = new AbortController(); request.current = controller;
    setBusy(true); onBusyChange(true); setError(null);
    try {
      const ref = await uploadFullDemoAsset(file, fullDemoProvenance(file, provenance), controller.signal);
      if (!controller.signal.aborted) {
        onUploaded(ref); setFile(null); setProvenance(EMPTY_PROVENANCE);
        if (fileInput.current) fileInput.current.value = '';
      }
    } catch (failure) { if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'No se pudo subir el archivo.'); }
    finally { if (!controller.signal.aborted) { setBusy(false); onBusyChange(false); } }
  }
  return <details open={open} className="border border-border-subtle p-3">
    <summary className="min-h-8 cursor-pointer text-body-sm font-medium text-primary">{label}</summary>
    <fieldset disabled={busy} className="mt-3 grid min-w-0 gap-3">
      <label className="text-body-sm text-fg-2" htmlFor={`${id}-file`}>Archivo local</label>
      <Input ref={fileInput} id={`${id}-file`} type="file" accept={accept} onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
      <details ref={rights} className="min-w-0">
        <summary className="min-h-8 cursor-pointer text-body-sm text-fg-2">Autoría y licencia (opcional)</summary>
        <div className="mt-2 grid min-w-0 gap-3">
          <p className="text-body-sm text-fg-2">Si lo dejas en blanco, se registra como archivo local aportado por ti, con el nombre del archivo como título y sin autor ni licencia declarados.</p>
          {([
            { key: 'title', label: 'Título', max: 200, placeholder: file?.name ?? '' }, { key: 'creator', label: 'Autor o titular', max: 200, placeholder: '' },
            { key: 'source_url', label: 'Fuente', max: 2000, placeholder: 'https://… o local:archivo-propio' },
            { key: 'permission', label: 'Licencia o permiso de uso', max: 4000, placeholder: '' }, { key: 'attribution', label: 'Texto de atribución, si corresponde', max: 4000, placeholder: '' },
          ] satisfies { key: keyof FullDemoProvenance; label: string; max: number; placeholder: string }[]).map((field) => {
            const fieldError = field.key === 'source_url' ? sourceError : null;
            return <div key={field.key} className="space-y-1.5">
              <label htmlFor={`${id}-${field.key}`} className="text-body-sm text-fg-2">{field.label}</label>
              <Input ref={field.key === 'source_url' ? sourceInput : undefined} id={`${id}-${field.key}`} maxLength={field.max} placeholder={field.placeholder} value={provenance[field.key]}
                aria-invalid={fieldError !== null} aria-describedby={fieldError ? `${id}-${field.key}-error` : undefined}
                onChange={(event) => { setProvenance({ ...provenance, [field.key]: event.target.value }); if (fieldError) setSourceError(null); }} />
              {fieldError ? <p id={`${id}-${field.key}-error`} role="alert" className="text-body-sm text-destructive">{fieldError}</p> : null}
            </div>;
          })}
          <p className="text-body-sm text-fg-2">El archivo se decodifica y se vincula a esta declaración. Cambiar el archivo o sus permisos requiere subir una nueva referencia.</p>
        </div>
      </details>
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      <Button type="button" disabled={file === null} loading={busy} loadingText="Verificando archivo…" onClick={() => void upload()}>Añadir archivo</Button>
    </fieldset>
  </details>;
}
