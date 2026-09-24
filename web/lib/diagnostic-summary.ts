// The "Copiar diagnóstico" block: the identifiers support needs to find this
// failure in the collector. It is copied by the user, never sent by the app.

export type DiagnosticSummaryInput = {
  supportCode?: string;
  sessionId?: string;
  jobId?: string;
  digest?: string;
  route?: string;
  message?: string;
};

export type DiagnosticLine = { label: string; value: string };

const LABELS: ReadonlyArray<[keyof DiagnosticSummaryInput, string]> = [
  ['supportCode', 'Código de soporte'],
  ['sessionId', 'Sesión'],
  ['jobId', 'Trabajo'],
  ['digest', 'Digest'],
  ['route', 'Ruta'],
  ['message', 'Mensaje'],
];

export function diagnosticLines(input: DiagnosticSummaryInput): DiagnosticLine[] {
  const lines: DiagnosticLine[] = [];
  for (const [key, label] of LABELS) {
    const value = input[key]?.trim();
    if (value) lines.push({ label, value });
  }
  return lines;
}

export function diagnosticText(lines: readonly DiagnosticLine[]): string {
  return lines.map((line) => `${line.label}: ${line.value}`).join('\n');
}
