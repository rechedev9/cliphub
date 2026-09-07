"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";

type Phase = "idle" | "uploading" | "error";

export function NewRequestForm() {
  const [note, setNote] = useState("");
  const [phase, setPhase] = useState<Phase>("idle");
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const router = useRouter();

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const file = fileInputRef.current?.files?.[0];
    if (!file) {
      setError("Selecciona un archivo .dem primero.");
      return;
    }

    setPhase("uploading");
    setError(null);

    try {
      const createRes = await fetch("/api/requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ note: note.trim() || undefined }),
      });
      if (!createRes.ok) {
        throw new Error("No se pudo crear la petición.");
      }
      const created = (await createRes.json()) as { id: string };

      const uploadRes = await fetch(`/api/requests/${created.id}/demo`, {
        method: "PUT",
        headers: { "X-Filename": encodeURIComponent(file.name) },
        body: file,
      });
      if (!uploadRes.ok) {
        const body = (await uploadRes.json().catch(() => null)) as {
          error?: string;
        } | null;
        throw new Error(body?.error ?? "Fallo al subir el archivo.");
      }

      setNote("");
      if (fileInputRef.current) fileInputRef.current.value = "";
      setPhase("idle");
      router.refresh();
    } catch (err) {
      setPhase("error");
      setError(err instanceof Error ? err.message : "Error desconocido.");
    }
  }

  return (
    <form className="card" onSubmit={handleSubmit}>
      <label htmlFor="demo-file">Archivo .dem</label>
      <input
        id="demo-file"
        ref={fileInputRef}
        type="file"
        accept=".dem"
        disabled={phase === "uploading"}
      />
      <label htmlFor="demo-note">Nota opcional</label>
      <textarea
        id="demo-note"
        placeholder="p.ej. soy el AWPer en el lado CT"
        maxLength={2000}
        value={note}
        onChange={(event) => setNote(event.target.value)}
        disabled={phase === "uploading"}
      />
      <button type="submit" disabled={phase === "uploading"}>
        {phase === "uploading" ? "Subiendo…" : "Enviar demo"}
      </button>
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}
