import { JOB_DEMO_MISSING_CODE, type LibrarySource } from '../api/library.ts';
import { SERVICE_UNAVAILABLE_CODE } from '../api/types.ts';
import { withoutDemoReason } from '../library/partidas.ts';
import { offlineMessage } from '../load-error.ts';

/** `codeSharedMatch` in internal/httpapi: the match came without its .dem. */
const SHARED_MATCH_CODE = 'shared_match';

export type CrearHandoffError = { title: string; description: string };

const TITLE = 'No se pudo pasar la demo a Crear';

function field(err: unknown, key: 'code' | 'status'): unknown {
  return typeof err === 'object' && err !== null && key in err ? Reflect.get(err, key) : undefined;
}

/** Why handing a library demo to Crear failed, and what to do next. */
export function crearHandoffError(err: unknown, source?: LibrarySource): CrearHandoffError {
  const code = field(err, 'code');
  const status = field(err, 'status');
  if (code === JOB_DEMO_MISSING_CODE) {
    return { title: TITLE, description: 'El archivo .dem de esta partida ya no está en este PC. Bórrala en Partidas y vuelve a importar el archivo para crear vídeos.' };
  }
  if (code === SHARED_MATCH_CODE) return { title: TITLE, description: withoutDemoReason(source) };
  if (code === SERVICE_UNAVAILABLE_CODE || err instanceof TypeError) {
    return { title: TITLE, description: offlineMessage() };
  }
  if (status === 404) return { title: TITLE, description: 'Esta partida ya no está en la biblioteca. Recarga la lista o vuelve a importar la demo.' };
  return { title: TITLE, description: 'Studio no pudo preparar la demo. Vuelve a intentarlo; si sigue fallando, reanaliza la partida en Partidas.' };
}
