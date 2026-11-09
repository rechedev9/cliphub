const MAX_FILE_STEM_LENGTH = 80;

/** A bare Windows-safe file name with this extension: no folders, no dot segments, no reserved characters. */
export function safeFileName(raw: string, kind: { extension: string; fallback: string }): string {
  const stem = raw
    .replace(new RegExp(`\\.${kind.extension}$`, 'i'), '')
    .split(/[\\/:*?"<>|\u0000-\u001f]+/)
    .map((part) => part.trim())
    .filter((part) => part !== '' && !/^\.+$/.test(part))
    .join(' ')
    .replace(/\s+/g, ' ')
    .replace(/^\.+/, '')
    .slice(0, MAX_FILE_STEM_LENGTH)
    .trim();
  return `${stem === '' ? kind.fallback : stem}.${kind.extension}`;
}
