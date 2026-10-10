import { test } from "node:test";
import assert from "node:assert/strict";

import { actionErrorText, eventDetailText, loadErrorText } from "./labels.ts";

test("event details the portal writes as English keys read in Spanish", () => {
  assert.equal(eventDetailText("created", "short, 436 s"), "Short, estimación de 7 min");
  assert.equal(eventDetailText("demo_uploaded", "300000 bytes"), "293 KB");
  assert.equal(eventDetailText("queued", "demo already stored"), "la demo ya estaba en el portal");
  assert.equal(eventDetailText("claimed", "attempt 2"), "intento 2");
  assert.equal(eventDetailText("stage", "rendering"), "Montando el vídeo");
  assert.equal(eventDetailText("phase_uploading", "412 s"), "7 min de máquina");
  assert.equal(eventDetailText("boosted", "front"), "subido al frente");
  assert.equal(eventDetailText("user_access_changed", "blocked"), "Bloqueado");
  assert.equal(
    eventDetailText("user_limits_changed", "maxActive 1, dailySeconds 1200"),
    "1 activo a la vez, 20 min de máquina al día",
  );
  assert.equal(
    eventDetailText("user_limits_changed", "maxActive default, dailySeconds default"),
    "activos a la vez por defecto, tiempo diario por defecto",
  );
});

test("a failure detail keeps the raw cause and only translates its code", () => {
  assert.equal(
    eventDetailText("failed", "render_failed: ffmpeg exit 1: Cannot allocate memory"),
    "Fallo de montaje: ffmpeg exit 1: Cannot allocate memory",
  );
  assert.equal(
    eventDetailText("worker_auto_paused", "capture_incompatible: recorder exit 6: AfxHookSource2"),
    "HLAE incompatible con CS2: recorder exit 6: AfxHookSource2",
  );
  assert.match(
    eventDetailText("requeued", "worker_lost: lease expired at 2026-10-09T19:07:13.573Z") ?? "",
    /^Worker perdido: su reserva caducó a las \d{2}:\d{2}$/,
  );
});

test("a job ended by the portal's own time limits says which limit", () => {
  assert.equal(
    eventDetailText("requeued", "worker_lost: running past its limit of 1440 s"),
    "Worker perdido: seguía en curso pasado su límite de 24 min",
  );
  assert.equal(
    eventDetailText("failed", "worker_lost: uploading past its limit of 23400 s"),
    "Worker perdido: seguía subiendo resultados pasado su límite de 6 h 30 min",
  );
});

test("an automatic pause reads with the title of the job that caused it, not its id", () => {
  assert.equal(
    eventDetailText("worker_auto_paused", 'disk_full: job 0b7c-11 "R3 4k": 2 GB left'),
    'Disco lleno: en "R3 4k": 2 GB left',
  );
  assert.equal(actionErrorText("code_confirmation_required").includes("ClipHub Studio"), true);
});

test("free text and details of unknown events are shown as written", () => {
  assert.equal(eventDetailText("worker_paused", "mantenimiento: cambio de disco"), "mantenimiento: cambio de disco");
  assert.equal(eventDetailText("canceled", "pedido por el usuario"), "pedido por el usuario");
  assert.equal(eventDetailText("device_linked", "DESKTOP-LUIS"), "DESKTOP-LUIS");
  assert.equal(eventDetailText("stage", "defragmenting"), "defragmenting");
  assert.equal(eventDetailText("done", null), null);
  assert.equal(eventDetailText("done", "  "), null);
});

test("a server failure reads as the portal's fault, with its status", () => {
  assert.equal(loadErrorText("http_500"), "El portal ha fallado al responder (error 500).");
  assert.match(actionErrorText("http_502"), /error 502\)\. No se ha hecho la acción/);
  assert.equal(loadErrorText("http_418"), "El portal respondió con un error (http_418).");
  assert.equal(actionErrorText("demo_gone"), "La demo ya no está en el portal, no se puede reintentar.");
});
