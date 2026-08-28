"""Generate glyphs.tsx from Streamline's FREE Core Solid set (#178, [202]).

Source: github.com/webalys-hq/streamline-vectors, core/solid, CC BY 4.0.
That is the free 1000-icon subset; the 5771-icon set on the website is premium
and its licence forbids redistribution, which a public repo would be.

Two things have to change on the way in:
  - fill="#000000" becomes currentColor, or the glyph ignores the theme and
    the rainbow engine and paints itself black in dark mode;
  - the <desc> block goes, since the button's label is the accessible name and
    a description here would be announced on top of it. The attribution it
    carries moves to the file header and the README, where a licence notice
    belongs and cannot be stripped by a build.
"""
import io
import os
import re

SRC = 'C:/Users/JUNKER~1/AppData/Local/Temp/slv/core/solid'

# our verb -> streamline file
WANTED = [
    ("IconSave", "computer-devices/floppy-disk.svg", "Save"),
    ("IconCancel", "interface-essential/delete-1.svg", "Cancel or dismiss"),
    ("IconRefresh", "interface-essential/arrow-reload-horizontal-1.svg", "Refresh or reload"),
    ("IconUpload", "interface-essential/upload-box-1.svg", "Upload or send"),
    ("IconSearch", "interface-essential/magnifying-glass.svg", "Search, scan or discover"),
    ("IconUnlock", "interface-essential/keyhole-lock-circle.svg", "Unlock, clear a stale lock"),
    ("IconPrune", "interface-essential/recycle-bin-2.svg", "Prune, reclaim space"),
    ("IconPlay", "entertainment/button-play.svg", "Start or run now"),
    ("IconStop", "entertainment/button-stop.svg", "Stop or abort"),
    ("IconBack", "interface-essential/move-left.svg", "Back or previous"),
    ("IconForward", "interface-essential/move-right.svg", "Next, continue or forward"),
    ("IconSelectAll", "interface-essential/check-square.svg", "Select all"),
    ("IconClearSelection", "interface-essential/subtract-square.svg", "Clear the selection"),
    ("IconKey", "interface-essential/key.svg", "Credentials"),
    ("IconLink", "interface-essential/link-chain.svg", "Connect or link"),
    ("IconEye", "interface-essential/glasses.svg", "Show, reveal or preview"),
    ("IconInfo", "interface-essential/information-circle.svg", "Information or details"),
]

HEADER = '''// ---------------------------------------------------------------------------
// Action glyphs (#178, [202]) — the symbols buttons wear.
//
// GENERATED from Streamline's free "Core Solid" set, see scripts/gen_glyphs.py.
// Do not hand-edit: regenerate instead, or the next run will overwrite the fix.
//
// Attribution (CC BY 4.0, required by the licence):
//   Free icons from Streamline — https://streamlinehq.com
//
// Only the FREE 1000-icon subset is used (github.com/webalys-hq/streamline-vectors,
// core/solid), which is CC BY 4.0 and explicitly redistributable. The larger
// 5771-icon set on the website is premium and its licence forbids
// redistribution, which is exactly what a public repository does.
//
// Sidebar.tsx keeps owning the NAVIGATION and domain glyphs. These are the
// VERBS the app has buttons for: save, cancel, refresh, upload, search,
// unlock, prune, play, stop, back, forward, select-all, clear-selection, key,
// link, eye, info.
//
// Two changes are made on import, both load-bearing:
//   - fill becomes `currentColor`, so a glyph inherits the ink its button
//     paints and therefore stays correct in every theme and in rainbow mode.
//     The source files hard-code #000000, which would be invisible on a dark
//     surface and would ignore the colour engine entirely.
//   - the source <desc> is dropped and `aria-hidden` added, because the
//     button's LABEL is the accessible name; a described glyph would be
//     announced on top of it.
//
// The source grid is 14 units, not the 16 Sidebar's own icons use, so the
// viewBox differs by design — both render into the same box.
// ---------------------------------------------------------------------------

import type { ReactNode } from "react";

function G({ children }: { children: ReactNode }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 14 14"
      fill="currentColor"
      className="shrink-0"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}
'''


def body(path):
    raw = io.open(SRC + "/" + path, encoding="utf-8").read()
    # drop the <desc> block, the outer <svg> wrapper and the id/desc noise
    raw = re.sub(r"<desc>.*?</desc>", "", raw, flags=re.S)
    inner = re.search(r"<svg[^>]*>(.*)</svg>", raw, re.S).group(1)
    inner = re.sub(r'\sid="[^"]*"', "", inner)
    inner = inner.replace('fill="#000000"', 'fill="currentColor"')
    inner = re.sub(r"\n\s*\n", "\n", inner).strip()
    # JSX needs camelCase attribute names
    for a, b in (
        ("fill-rule", "fillRule"),
        ("clip-rule", "clipRule"),
        ("stroke-width", "strokeWidth"),
        ("stroke-linecap", "strokeLinecap"),
        ("stroke-linejoin", "strokeLinejoin"),
        ("stroke-miterlimit", "strokeMiterlimit"),
    ):
        inner = inner.replace(a + "=", b + "=")
    return inner


out = [HEADER]
for name, path, note in WANTED:
    out.append("\n/** %s. */\nexport function %s() {\n  return (\n    <G>\n%s\n    </G>\n  );\n}\n" % (note, name, body(path)))

io.open("src/components/glyphs.tsx", "w", encoding="utf-8", newline="").write("".join(out))
print("wrote", len(WANTED), "glyphs from the free Streamline Core Solid set")
