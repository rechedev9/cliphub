"""One-shot TickCut -> ClipHub rebrand, including the Go module path.

Unlike scripts/rebrand-tickcut.py (the earlier FragForge -> TickCut pass, which
deliberately preserved infrastructure identifiers), this one also rewrites the Go
module path, the GitHub repository slug, the landing domain and the Vercel project.
Those three are external resources: rename them upstream before trusting the URLs.

The published v2.4.21 installer URL is protected on purpose. Its asset filename
lives on an existing GitHub release and cannot be renamed retroactively, so the
landing download must keep pointing at it until a ClipHub-named release exists.
"""
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

# Historical record of the previous rebrand; rewriting it would make it describe a
# migration that never happened.
SKIP_FILES = {"rebrand-tickcut.py", "rebrand-cliphub.py"}

EXTS = {
    ".cjs",
    ".cmd",
    ".css",
    ".example",
    ".go",
    ".html",
    ".js",
    ".json",
    ".lua",
    ".md",
    ".mjs",
    ".mod",
    ".ps1",
    ".py",
    ".rhai",
    ".sh",
    ".svg",
    ".ts",
    ".tsx",
    ".txt",
    ".yaml",
    ".yml",
}

PROTECT = [
    (
        "https://github.com/rechedev9/tickcut/releases/download/v2.4.21/"
        "TickCut.Studio.Setup.2.4.21.exe",
        "@@PUBLISHED_INSTALLER@@",
    ),
]

REPLACEMENTS = [
    ("github.com/rechedev9/tickcut", "github.com/rechedev9/cliphub"),
    ("rechedev9/tickcut", "rechedev9/cliphub"),
    ("tickcut.gravityroom.app", "cliphub.gravityroom.app"),
    ("TICKCUT.GRAVITYROOM.APP", "CLIPHUB.GRAVITYROOM.APP"),
    ("tickcut-landing", "cliphub-landing"),
    ("TickCut Studio", "ClipHub Studio"),
    ("tickcut-studio", "cliphub-studio"),
    ("com.tickcut.studio", "com.cliphub.studio"),
    ("TickCut.Studio.Setup", "ClipHub.Studio.Setup"),
    ("TickCut-Studio-build", "ClipHub-Studio-build"),
    ("X-TickCut-Token", "X-ClipHub-Token"),
    ("X-TickCut-", "X-ClipHub-"),
    ("tickcut_proxy_capability", "cliphub_proxy_capability"),
    ("tickcutSettings", "cliphubSettings"),
    ("tickcutClipboard", "cliphubClipboard"),
    ("TICKCUTAssistant", "CLIPHUBAssistant"),
    ("tickcut-mark.svg", "cliphub-mark.svg"),
    ("tickcut-icon.jpg", "cliphub-icon.jpg"),
    ("tickcut-wordmark.jpg", "cliphub-wordmark.jpg"),
    ("TickCut", "ClipHub"),
    ("TICKCUT_", "CLIPHUB_"),
    ("TICKCUT", "CLIPHUB"),
]


def main() -> None:
    changed: list[str] = []
    for path in ROOT.rglob("*"):
        if not path.is_file():
            continue
        if any(part in SKIP_PARTS for part in path.parts):
            continue
        if path.name in SKIP_FILES:
            continue
        if path.suffix.lower() not in EXTS:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        if not re.search("tickcut", text, re.IGNORECASE):
            continue
        orig = text
        for src, token in PROTECT:
            text = text.replace(src, token)
        for a, b in REPLACEMENTS:
            text = text.replace(a, b)
        # Remaining bare product tokens (identifiers, temp-dir prefixes, hostnames).
        text = text.replace("tickcut", "cliphub")
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
