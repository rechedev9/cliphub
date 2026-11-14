import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";

// Reads the home page exactly as `next build` prerendered it, so it checks what a visitor receives.
// The test script runs from the landing folder.
const html = readFileSync(path.join(".next", "server", "app", "index.html"), "utf8");
const site = readFileSync(path.join("lib", "site.ts"), "utf8");

function constant(name: string): string {
  const match = new RegExp(`${name} =\\s*"([^"]+)"`).exec(site);
  assert.ok(match, `${name} is missing from lib/site.ts`);
  return match[1] ?? "";
}

function attr(tag: string, name: string): string | undefined {
  return new RegExp(`\\s${name}="([^"]*)"`).exec(tag)?.[1];
}

test("the hero has the headline and both calls to action", () => {
  assert.match(html, /<h1[^>]*>Lee al rival antes de que juegue\.<\/h1>/);
  assert.ok(html.includes(`href="${constant("ANALYTICS_URL")}"`), "no link to ClipHub Analytics");
  assert.ok(html.includes(`href="${constant("DOWNLOAD_URL")}"`), "no link to the installer");
  assert.ok(html.includes("Abrir en el navegador"));
  assert.ok(html.includes("Descargar para Windows"));
});

test("every agreed section exists and the nav reaches it", () => {
  for (const id of ["analisis", "comms", "equipo", "comparativa", "creadores", "descargar"]) {
    assert.match(html, new RegExp(`<section[^>]*id="${id}"`), `section #${id} is missing`);
  }
  for (const id of ["analisis", "comms", "equipo", "creadores", "descargar"]) {
    assert.ok(html.includes(`href="#${id}"`), `nav link to #${id} is missing`);
  }
});

test("the page names no competitor and no old brand", () => {
  const text = html.toLowerCase();
  for (const name of ["cs2lens", "csgolens", "leetify", "refrag", "scope.gg", "clipmind"]) {
    assert.ok(!text.includes(name), `the page mentions ${name}`);
  }
});

test("every image has a descriptive alt or is marked decorative", () => {
  const images = html.match(/<img\b[^>]*>/g) ?? [];
  assert.ok(images.length >= 8, `only ${images.length} images`);
  for (const tag of images) {
    const alt = attr(tag, "alt");
    assert.notEqual(alt, undefined, `image without alt: ${tag}`);
    if (attr(tag, "src")?.startsWith("/shots/")) assert.ok((alt ?? "").length > 40, `short alt on ${tag}`);
  }
});

test("the metadata is in Spanish and filled in", () => {
  assert.match(html, /<html lang="es"/);
  assert.match(html, /<title>ClipHub: análisis de CS2[^<]+<\/title>/);
  assert.match(html, /<meta name="description" content="[^"]{60,}"/);
  assert.match(html, /<meta property="og:title" content="ClipHub[^"]+"/);
  assert.match(html, /<meta property="og:image" content="[^"]+opengraph-image[^"]*"/);
});
