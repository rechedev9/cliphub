import type { Metadata } from "next";

import { SiteFooter } from "../site-footer";

export const metadata: Metadata = {
  title: "Política de Privacidad",
};

const CONTACT_EMAIL = process.env.CONTACT_EMAIL ?? "rsnoverwatch@gmail.com";

export default function PrivacyPage() {
  return (
    <main className="narrow doc">
      <h1>Política de Privacidad</h1>
      <p className="updated">Última actualización: 7 de septiembre de 2026</p>

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
          <strong>Lo que envías.</strong> El archivo <code>.dem</code> de la
          partida y la nota opcional que escribas.
        </li>
        <li>
          <strong>El resultado.</strong> El vídeo generado a partir de tu demo.
        </li>
        <li>
          <strong>Una cookie de sesión</strong> para mantenerte identificado
          mientras usas el sitio.
        </li>
      </ul>

      <h2>Para qué los usamos</h2>
      <p>
        Únicamente para prestar el servicio: identificarte, mostrarte tus
        propias peticiones y producir y entregarte el vídeo. No hay publicidad,
        no hay analítica de terceros, no hacemos perfilado y no vendemos ni
        cedemos tus datos a nadie.
      </p>

      <h2>Dónde se procesa tu demo</h2>
      <p>
        La edición requiere el propio juego, así que tu demo se transfiere desde
        el servidor al ordenador personal del operador, en España, donde se
        graba y se monta el vídeo. La copia que hay en el servidor se borra en
        cuanto esa transferencia se completa: no mantenemos dos copias de tu
        demo.
      </p>

      <h2>Cuánto tiempo los conservamos</h2>
      <ul>
        <li>
          <strong>Tu demo:</strong> se borra del servidor en cuanto pasa al
          ordenador donde se procesa.
        </li>
        <li>
          <strong>El vídeo entregado:</strong> se borra automáticamente a los 30
          días de la entrega. Descárgalo antes de ese plazo.
        </li>
        <li>
          <strong>Tu cuenta y el historial de peticiones:</strong> se conservan
          mientras uses el servicio, hasta que pidas su supresión.
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

      <SiteFooter />
    </main>
  );
}
