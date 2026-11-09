/** The Crear formats that carry their own colour, in the flows and in Mis vídeos. */
export type FormatTone = 'short' | 'full' | 'stream' | 'summary';

type ToneClasses = {
  /** A solid fill with readable text on it: primary buttons, chips, step numbers. */
  readonly fill: string;
  /** The ground of a hero band or a card in the flow; neutral, so the format colour stays on fills, bars and text. */
  readonly tint: string;
  readonly border: string;
  readonly text: string;
  /** The bare colour for a bar, a dot or a track segment. */
  readonly bg: string;
};

export const FORMAT_TONE: Readonly<Record<FormatTone, ToneClasses>> = {
  short: {
    fill: 'bg-format-short text-format-short-foreground hover:bg-format-short/90',
    tint: 'bg-surface-2',
    border: 'border-border',
    text: 'text-format-short',
    bg: 'bg-format-short',
  },
  full: {
    fill: 'bg-format-full text-format-full-foreground hover:bg-format-full/90',
    tint: 'bg-surface-2',
    border: 'border-border',
    text: 'text-format-full',
    bg: 'bg-format-full',
  },
  stream: {
    fill: 'bg-format-stream text-format-stream-foreground hover:bg-format-stream/90',
    tint: 'bg-surface-2',
    border: 'border-border',
    text: 'text-format-stream',
    bg: 'bg-format-stream',
  },
  summary: {
    fill: 'bg-format-summary text-format-summary-foreground hover:bg-format-summary/90',
    tint: 'bg-surface-2',
    border: 'border-border',
    text: 'text-format-summary',
    bg: 'bg-format-summary',
  },
};
