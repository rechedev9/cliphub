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

test("the hero has the headline and the download", () => {
  assert.match(html, /<h1[^>]*>Tu demo, hecha vídeo\.<\/h1>/);
  assert.ok(html.includes(`href="${constant("DOWNLOAD_URL")}"`), "no link to the installer");
  assert.ok(html.includes("Descargar para Windows"));
  assert.ok(html.includes(constant("RELEASE_VERSION")), "the release version is not shown");
});

test("every agreed section exists and the nav reaches it", () => {
  for (const id of ["como-funciona", "shorts", "demo-completa", "streams", "preguntas", "descargar"]) {
    assert.match(html, new RegExp(`<section[^>]*id="${id}"`), `section #${id} is missing`);
  }
  for (const id of ["shorts", "demo-completa", "streams", "preguntas"]) {
    assert.ok(html.includes(`href="#${id}"`), `nav link to #${id} is missing`);
  }
});

test("the page sells only Studio and never links to the web analysis", () => {
  assert.ok(!html.includes("cliphubstrats.rechedev.cloud"), "the page links to the web edition");
});

test("every looping clip is described and muted", () => {
  const videos = html.match(/<video\b[^>]*>/g) ?? [];
  assert.ok(videos.length >= 2, `only ${videos.length} clips`);
  for (const tag of videos) {
    assert.ok((attr(tag, "aria-label") ?? "").length > 40, `clip without a description: ${tag}`);
    assert.match(tag, /\smuted(\s|=|>)/, `clip with sound: ${tag}`);
  }
});

test("the page names no competitor and no old brand", () => {
  const text = html.toLowerCase();
  for (const name of ["cs2lens", "csgolens", "leetify", "refrag", "scope.gg", "clipmind"]) {
    assert.ok(!text.includes(name), `the page mentions ${name}`);
  }
});

test("every image has an alt", () => {
  const images = html.match(/<img\b[^>]*>/g) ?? [];
  for (const tag of images) {
    const alt = attr(tag, "alt");
    assert.notEqual(alt, undefined, `image without alt: ${tag}`);
  }
});

test("the metadata is in Spanish and filled in", () => {
  assert.match(html, /<html lang="es"/);
  assert.match(html, /<title>Clips y Shorts de CS2[^<]*ClipHub Studio<\/title>/);
  assert.match(html, /<meta name="description" content="[^"]{60,}"/);
  assert.match(html, /<meta property="og:title" content="[^"]*ClipHub Studio"/);
  assert.match(html, /<meta property="og:image" content="[^"]+opengraph-image[^"]*"/);
});

test("search engines get the app and its questions as structured data", () => {
  const block = /<script type="application\/ld\+json">([^<]+)<\/script>/.exec(html);
  assert.ok(block?.[1], "no JSON-LD block");
  const data: unknown = JSON.parse(block[1]);
  assert.ok(Array.isArray(data));
  const types = data.map((entry) => (typeof entry === "object" && entry !== null && "@type" in entry ? entry["@type"] : ""));
  assert.deepEqual(types, ["SoftwareApplication", "FAQPage"]);
});
