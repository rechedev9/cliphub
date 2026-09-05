/** A readable cross-platform filename. Paths and control characters never reach the save dialog. */
export function streamVideoFilename(title: string): string {
  const stem =
    title
      .normalize('NFC')
      .replace(/[<>:"/\\|?*\u0000-\u001f\u007f]/g, '-')
      .replace(/[. ]+$/g, '')
      .trim()
      .slice(0, 100)
      .replace(/[. ]+$/g, '') || 'Short';
  return `${/^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(stem) ? 'Short-' : ''}${stem}.mp4`;
}
