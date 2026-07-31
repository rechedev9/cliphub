"""One-shot FragForge -> TickCut user-facing rebrand. Preserves Go module path."""
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

SKIP_PARTS = {
    "node_modules",
    ".git",
    "bin",
    "data",
    "dist",
    "dist-installer",
    "build-resources",
    ".next",
    "test-results",
    "win-unpacked",
    "remotion-assets",
    "vendor",
    ".dist-installer-stage",
}

EXTS = {
    ".ts",
    ".tsx",
    ".js",
    ".mjs",
    ".cjs",
    ".json",
    ".md",
    ".css",
    ".html",
    ".svg",
    ".go",
    ".ps1",
    ".sh",
    ".yml",
    ".yaml",
    ".rhai",
}

REPLACEMENTS = [
    ("FragForge Studio", "TickCut Studio"),
    ("fragforge-studio", "tickcut-studio"),
    ("com.fragforge.studio", "com.tickcut.studio"),
    ("FragForge.Studio.Setup", "TickCut.Studio.Setup"),
    ("X-FragForge-Token", "X-TickCut-Token"),
    ("fragforge_proxy_capability", "tickcut_proxy_capability"),
    ("fragforgeSettings", "tickcutSettings"),
    ("fragforgeClipboard", "tickcutClipboard"),
    ("fragforge:clipboard-write", "tickcut:clipboard-write"),
    ("fragforge.reels.v1", "tickcut.reels.v1"),
    ("fragforge.stream-draft.", "tickcut.stream-draft."),
    ("fragforge.uploads.v1", "tickcut.uploads.v1"),
    ("fragforge.session.v1", "tickcut.session.v1"),
    ("fragforge:news-short", "tickcut:news-short"),
    ("fragforge:sw-evicted", "tickcut:sw-evicted"),
    ("[fragforge]", "[tickcut]"),
    ("FragForge", "TickCut"),
    ("FRAGFORGE_", "TICKCUT_"),
]

PROTECT = [
    ("github.com/rechedev9/fragforge", "@@GOMODULE@@"),
    ("rechedev9/fragforge", "@@GHREPO@@"),
    ("fragforge.gravityroom.app", "@@DOMAIN@@"),
    ("fragforge-landing", "@@VERCEL_PROJECT@@"),
]


def main() -> None:
    changed: list[str] = []
    for path in ROOT.rglob("*"):
        if not path.is_file():
            continue
        if any(part in SKIP_PARTS for part in path.parts):
            continue
        if path.suffix.lower() not in EXTS:
            continue
        # Don't rewrite this script mid-run.
        if path.name == "rebrand-tickcut.py":
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except OSError:
            continue
        if "FragForge" not in text and "fragforge" not in text and "FRAGFORGE" not in text:
            continue
        orig = text
        for src, token in PROTECT:
            text = text.replace(src, token)
        for a, b in REPLACEMENTS:
            text = text.replace(a, b)
        # Remaining bare fragforge product tokens (not protected placeholders).
        text = re.sub(r"\bfragforge\b", "tickcut", text)
        for src, token in PROTECT:
            text = text.replace(token, src)
        if text != orig:
            path.write_text(text, encoding="utf-8", newline="\n")
            changed.append(str(path.relative_to(ROOT)))
    print(f"updated {len(changed)} files")
    for p in changed:
        print(p)


if __name__ == "__main__":
    main()
