export type StreamMediaFailure = { readonly detail: string; readonly message: string };

const MEDIA_ERROR_MESSAGES: Readonly<Record<number, string>> = {
  1: 'La carga del vídeo se interrumpió. Vuelve a intentar la vista previa.',
  2: 'No se pudo leer el MP4 de origen. Comprueba que el archivo siga en su carpeta y vuelve a intentarlo.',
  3: 'El MP4 de origen está dañado o no se puede decodificar. Prueba a importarlo otra vez.',
  4: 'La vista previa no puede reproducir el formato de este vídeo.',
};

const UNKNOWN_MEDIA_MESSAGE = 'No se pudo reproducir el MP4 de origen. Vuelve a intentar la vista previa.';

export const STREAM_AUDIO_FAILURE: StreamMediaFailure = {
  detail: 'audio mixer failed',
  message: 'No se pudo preparar el audio de la vista previa. Vuelve a intentarlo.',
};

export const STREAM_AUDIO_RESUME_FAILURE: StreamMediaFailure = {
  detail: 'audio resume failed',
  message: 'No se pudo reanudar el audio de la vista previa. Pulsa reproducir otra vez.',
};

export function streamMediaFailure(error: { readonly code: number; readonly message: string } | null): StreamMediaFailure {
  if (error === null) return { detail: 'MediaError unknown', message: UNKNOWN_MEDIA_MESSAGE };
  return {
    detail: `MediaError ${error.code}: ${error.message || 'no message'}`,
    message: MEDIA_ERROR_MESSAGES[error.code] ?? UNKNOWN_MEDIA_MESSAGE,
  };
}
