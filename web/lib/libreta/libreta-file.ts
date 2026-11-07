import { importStratbook } from '../api/library.ts';
import { importSummary, parseStratbookFile } from '../stratbook/plays.ts';
import { CALLOUT_NAMES } from './callouts.ts';

/** Imports the text of a plays file or a whole Libreta file and says, in Spanish, what each section did. */
export async function importLibretaText(text: string): Promise<string> {
  const parsed = parseStratbookFile(text);
  if (!parsed.ok) return parsed.error;
  try {
    const result = await importStratbook(parsed.file);
    if ((result.sections?.callout_names?.imported ?? 0) > 0) CALLOUT_NAMES.reset();
    return importSummary(result);
  } catch {
    return 'No se pudo importar el fichero. Comprueba que lo exportó Studio.';
  }
}
