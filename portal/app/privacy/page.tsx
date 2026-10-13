import type { Metadata } from "next";

import { DocPage } from "@/components/doc-page";

export const metadata: Metadata = {
  title: "Política de Privacidad",
};

const CONTACT_EMAIL = process.env.CONTACT_EMAIL ?? "rsnoverwatch@gmail.com";

export default function PrivacyPage() {
  return (
    <DocPage title="Política de Privacidad" updated="Última actualización: 10 de octubre de 2026">

      <h2>Quién trata tus datos</h2>
      <p>
        ClipHub es un servicio personal y gratuito operado por un particular.
        Para cualquier asunto relacionado con tus datos puedes escribir a{" "}
        <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>.
      </p>

      <h2>Qué datos recogemos</h2>
      <ul>
        <li>
          <strong>De tu cuenta.</strong> Cuando inicias sesión con Google o
          Discord recibimos tu nombre, tu dirección de correo, la URL de tu foto
          de perfil y el identificador que ese proveedor asigna a tu cuenta. No
          recibimos ni almacenamos tu contraseña.
        </li>
        <li>
          <strong>De tus dispositivos.</strong> Al vincular ClipHub Studio
          guardamos el nombre del dispositivo y las fechas de uso. Para avisarte
          si la vinculación se pide desde otra red, guardamos solo una huella
          irreversible de la dirección de red, no la dirección.
        </li>
        <li>
          <strong>Lo que envías a la nube.</strong> El archivo <code>.dem</code>{" "}
          de la partida y los ajustes del vídeo que Studio envía con cada
          trabajo.
        </li>
        <li>
          <strong>El resultado.</strong> Los vídeos generados a partir de tu
          demo.
        </li>
        <li>
          <strong>Un registro de actividad</strong> de cada trabajo (cuándo se
          creó, se grabó, falló o terminó y por qué), que usa el operador para
          mantener el servicio.
        </li>
        <li>
          <strong>Una cookie de sesión</strong> para mantenerte identificado
          mientras usas el sitio.
        </li>
      </ul>

      <h2>Para qué los usamos</h2>
      <p>
        Únicamente para prestar el servicio: identificarte, vincular Studio,
        mostrarte tus propios trabajos y producir y entregarte el vídeo. No hay
        publicidad, no hay analítica de terceros, no hacemos perfilado y no
        vendemos ni cedemos tus datos a nadie.
      </p>

      <h2>Dónde se procesa tu demo</h2>
      <p>
        La grabación requiere el propio juego, así que tu demo se transfiere
        desde el servidor al equipo de ClipHub que la graba, en España. El
        vídeo resultante vuelve al servidor para que puedas descargarlo.
      </p>

      <h2>Cuánto tiempo los conservamos</h2>
      <ul>
        <li>
          <strong>Tu demo:</strong> se borra del servidor cuando ningún trabajo
          tuyo la necesita; si un trabajo falla, se conserva como máximo 48
          horas.
        </li>
        <li>
          <strong>El vídeo terminado:</strong> se borra 24 horas después de que
          Studio confirme que lo recibió, o a los 7 días de generarse si no lo
          confirmó. Descárgalo antes de ese plazo.
        </li>
        <li>
          <strong>El registro de actividad:</strong> se conserva 90 días.
        </li>
        <li>
          <strong>Tu cuenta, tus dispositivos y el historial de trabajos:</strong>{" "}
          se conservan mientras uses el servicio, hasta que pidas su supresión.
        </li>
      </ul>

      <h2>Tus derechos</h2>
      <p>
        Puedes solicitar el acceso, la rectificación o la supresión de tus
        datos, así como la eliminación completa de tu cuenta, escribiendo a{" "}
        <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>. Atenderemos la
        solicitud en un plazo razonable.
      </p>

      <h2>Cambios</h2>
      <p>
        Si esta política cambia, la versión actualizada se publicará en esta
        misma página con su nueva fecha.
      </p>
    </DocPage>
  );
}
