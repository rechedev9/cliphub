# ClipMind (web edition)

The web edition of the analysis: accounts, team workspaces, a browser import that sends only the analysis, and the same analysis pages as Studio. The browser is the real client here, so every journey runs through the UI. How to run it locally: [the skill](../SKILL.md#clipmind-web-edition).

Last proven: 2026-11-19 on `83ce9fff`: sign-in through the form, browser import of a `.dem` and of a `.dem.gz`, match page.

## Sub-features

- Account pages: `/crear-cuenta` ("Crea tu cuenta"), `/iniciar-sesion` ("Entra en ClipMind"), `/confirmar`, `/recuperar`, `/restablecer`, `/invitacion`.
- `/importar`: the browser scans each demo with the Go scanner compiled to WebAssembly and uploads only the analysis.
- The rail has no Crear group. Its foot has "Importar demos", "Equipo" (`/equipo`, workspaces, members, invitations), "Cuenta" (`/cuenta`, sessions, assistant keys), "Ajustes" and "Descargar Studio".
- The match page has no "Vídeos" tab and no "Crear Short" or "Demo completa" buttons.
- Studio sync: Studio's Ajustes, section "Cuenta ClipMind", signs in to a ClipMind server and syncs its library with the workspace.

## How to get to it (user POV)

1. Open any page signed out. `/partidas` redirects to `/iniciar-sesion?next=%2Fpartidas`.
2. Fill "Usuario o correo" and "Contraseña", then "Entrar". `/partidas` opens with "Aún no hay partidas".
3. "Importar demos", then "Elegir demos". Each demo gets a row whose state ends at "Importada" (the code also has "En cola", "Analizando", "Esperando para subir" and "Subiendo"; a 51 MB demo passed them in under 3 s).
4. "Importación terminada: 1 partida nueva." appears with "Ver partidas". The match opens from the list as in Studio.

## Driving it with the browser

- Sign-in fields have no name attribute: use `role=textbox` "Usuario o correo", `input[type=password]` and `role=button` "Entrar".
- The file input is `input[aria-label="Demos para importar"]` (accepts `.dem`, `.zst`, `.gz`). Set the file on it with the browser tool's upload action.
- Wait for the text "Importación terminada". The row reads "<file> · <size> · <Mapa>" and "Importada", with the link "Abrir la partida".
- The match page is the same as in [Partidas](partidas.md), with the tabs "Resumen", "Rondas", "Replay", "Métricas".

Proof: before, "Aún no hay partidas"; after, "Importación terminada: 1 partida nueva." and the match page (proven: "Vertigo", "8 : 8", "16 rondas"). Side effect: `<data dir>\workspaces\<workspace>\data\library\<demo>\` holds `positions.zvpos` and `tactical.json` and no `.dem` (1.8 MB in total for a 51 MB demo).

## Gotchas

- Cookies ignore the port, so the session cookie of an earlier run on `127.0.0.1` survives in the browser. The new server rejects it: `/partidas` loads without a redirect and shows "No se pudo leer la biblioteca", with 401 from `/api/library/*`. Open `/iniciar-sesion?next=%2Fpartidas` and sign in, which replaces the cookie.
- A browser tool can cap uploads (100 MB in OpenBot's browser on 2026-11-19), and FACEIT demos are 150 to 200 MB. Keep a test demo under the cap; a `.dem.gz` is smaller than the `.dem`.
- `clipmind-server user add` makes a confirmed account, so no mail is needed. With `CLIPMIND_MAIL=log` (the default) a sign-up through `/crear-cuenta` prints the confirmation link in the server log.
- Sign-up is limited to 5 per 10 minutes per server.
- The match source reads "Sin identificar" for a demo whose source the scanner could not tell.
- Linking Studio to a workspace needs the Studio window (no MCP tool signs in), so this skill has not proven sync. Plain `http` is accepted only for a localhost server (`http://127.0.0.1:<api>`). A 409 on sync means Studio and ClipMind run different commits.
