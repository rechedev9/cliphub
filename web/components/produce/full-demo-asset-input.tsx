'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { uploadFullDemoAsset, type FullDemoAssetRef, type FullDemoProvenance } from '@/lib/full-demo-plan';

export function FullDemoAssetInput({ label, accept, onUploaded, onBusyChange }: {
  label: string; accept: string; onUploaded: (ref: FullDemoAssetRef, title: string) => void; onBusyChange: (busy: boolean) => void;
}): ReactNode {
  const id = useId();
  const [file, setFile] = useState<File | null>(null);
  const [provenance, setProvenance] = useState<FullDemoProvenance>({ title: '', creator: '', source_url: '', permission: '', attribution: '' });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  const ready = file !== null && provenance.title.trim() !== '' && provenance.creator.trim() !== '' && provenance.source_url.trim() !== '' && provenance.permission.trim() !== '';
  async function upload(): Promise<void> {
    if (!ready || !file) return;
    const controller = new AbortController(); request.current = controller;
    setBusy(true); onBusyChange(true); setError(null);
    try {
      const ref = await uploadFullDemoAsset(file, provenance, controller.signal);
      if (!controller.signal.aborted) { onUploaded(ref, provenance.title); setFile(null); }
    } catch (failure) { if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'No se pudo subir el archivo.'); }
    finally { if (!controller.signal.aborted) { setBusy(false); onBusyChange(false); } }
  }
  return <details className="border border-border-subtle p-3">
    <summary className="min-h-8 cursor-pointer text-body-sm font-medium text-primary">{label}</summary>
    <fieldset disabled={busy} className="mt-3 grid min-w-0 gap-3">
      <label className="text-body-sm text-fg-2" htmlFor={`${id}-file`}>Archivo local</label>
      <Input id={`${id}-file`} type="file" accept={accept} onChange={(event) => {
        const selected = event.target.files?.[0] ?? null; setFile(selected);
        if (selected && provenance.title === '') setProvenance({ ...provenance, title: selected.name });
      }} />
      {([
        { key: 'title', label: 'Título', max: 200 }, { key: 'creator', label: 'Autor o titular', max: 200 },
        { key: 'source_url', label: 'Fuente (https://… o local:archivo-propio)', max: 2000 },
        { key: 'permission', label: 'Licencia o permiso de uso', max: 4000 }, { key: 'attribution', label: 'Texto de atribución, si corresponde', max: 4000 },
      ] satisfies { key: keyof FullDemoProvenance; label: string; max: number }[]).map((field) => <div key={field.key} className="space-y-1.5">
        <label htmlFor={`${id}-${field.key}`} className="text-body-sm text-fg-2">{field.label}</label>
        <Input id={`${id}-${field.key}`} maxLength={field.max} value={provenance[field.key]} onChange={(event) => setProvenance({ ...provenance, [field.key]: event.target.value })} />
      </div>)}
      <p className="text-meta text-fg-3">El archivo se decodifica y se vincula a esta declaración. Cambiar el archivo o sus permisos requiere subir una nueva referencia.</p>
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      <Button type="button" disabled={!ready} loading={busy} loadingText="Verificando archivo…" onClick={() => void upload()}>Añadir archivo</Button>
    </fieldset>
  </details>;
}
