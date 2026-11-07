/** The Crear formats that carry their own colour, in the flows and in Mis vídeos. */
export type FormatTone = 'short' | 'full' | 'stream' | 'summary';

type ToneClasses = {
  /** A solid fill with readable text on it: primary buttons, chips, step numbers. */
  readonly fill: string;
  /** The tinted ground of a hero band or a selected card. */
  readonly tint: string;
  readonly border: string;
  readonly text: string;
  /** The bare colour for a bar, a dot or a track segment. */
  readonly bg: string;
};

export const FORMAT_TONE: Readonly<Record<FormatTone, ToneClasses>> = {
  short: {
    fill: 'bg-format-short text-format-short-foreground hover:bg-format-short/90',
    tint: 'bg-format-short/12',
    border: 'border-format-short/45',
    text: 'text-format-short',
    bg: 'bg-format-short',
  },
  full: {
    fill: 'bg-format-full text-format-full-foreground hover:bg-format-full/90',
    tint: 'bg-format-full/12',
    border: 'border-format-full/45',
    text: 'text-format-full',
    bg: 'bg-format-full',
  },
  stream: {
    fill: 'bg-format-stream text-format-stream-foreground hover:bg-format-stream/90',
    tint: 'bg-format-stream/12',
    border: 'border-format-stream/45',
    text: 'text-format-stream',
    bg: 'bg-format-stream',
  },
  summary: {
    fill: 'bg-format-summary text-format-summary-foreground hover:bg-format-summary/90',
    tint: 'bg-format-summary/12',
    border: 'border-format-summary/45',
    text: 'text-format-summary',
    bg: 'bg-format-summary',
  },
};
