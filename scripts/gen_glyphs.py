"""Generate the app's glyphs from Streamline's FREE Core Solid set (#178, [202]).

Run from web/:  python ../scripts/gen_glyphs.py <path-to-streamline-vectors/core/solid>

Source: github.com/webalys-hq/streamline-vectors, folder core/solid, CC BY 4.0.
That is the free 1000-icon subset, and its README says redistribution is
encouraged as long as the licence terms are followed. The 5771-icon set on
streamlinehq.com is a DIFFERENT, PREMIUM product whose licence forbids
redistribution, which is precisely what a public repository does; it must not
be used here.

Writes two files:
  src/components/glyphs.tsx      the action verbs buttons wear
  src/components/navGlyphs.tsx   the navigation and domain symbols

Sidebar.tsx re-exports from navGlyphs, so the many files importing IconTrash
and friends keep working untouched.
"""
import io
import re
import sys

SRC = sys.argv[1] if len(sys.argv) > 1 else "/tmp/slv/core/solid"

# our name -> (streamline file, what it means here)
ACTION = [
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

# The navigation and domain set Sidebar.tsx used to draw by hand. Same source
# now, so the whole interface reads as one icon family rather than two.
NAV = [
    ("IconVM", "computer-devices/screen-1.svg", "Virtual machines"),
    ("IconFiles", "interface-essential/new-folder.svg", "Files and folder sets"),
    ("IconReceiver", "interface-essential/login-1.svg", "Receiver, an incoming transfer"),
    ("IconFleet", "interface-essential/hierarchy-2.svg", "Fleet, other BombVault boxes"),
    ("IconFolder", "interface-essential/new-folder.svg", "A folder"),
    # "Local", as opposed to off-site. Its own symbol rather than the folder
    # (jdp: "Ist ja blöd wenn buttons mit unterschiedlicher funktion das gleiche
    # symbol tragen" — the Local segment and the Browse button both wore a
    # folder and mean different things). A drive against a cloud is the pairing
    # everyone already reads, and neither half is a folder.
    #
    # hard-disk rather than hard-drive-1, jdp's pick from the sheet ([242]). It
    # is scaled to the cloud's envelope on the way out — see FIT_TO_CLOUD, and
    # note the entry there names THIS file: swapping the source without
    # re-measuring would silently scale the wrong ink box.
    ("IconLocal", "computer-devices/hard-disk.svg", "Local storage, as opposed to off-site"),
    ("IconAdd", "interface-essential/add-1.svg", "Add"),
    ("IconDownload", "interface-essential/download-box-1.svg", "Download or export"),
    ("IconBackupNow", "computer-devices/database-check.svg", "Back up now"),
    ("IconRestore", "interface-essential/arrow-reload-vertical-1.svg", "Restore"),
    ("IconPower", "entertainment/button-power-1.svg", "Power, start or stop"),
    ("IconLive", "interface-essential/live-video.svg", "Live, currently running"),
    ("IconTrash", "interface-essential/recycle-bin-2.svg", "Delete"),
    ("IconPencil", "interface-essential/pencil.svg", "Edit"),
    ("IconCheckCircle", "interface-essential/shield-check.svg", "Verified or test connection"),
    # Moved off arrow-reload-horizontal-2 when Recovery took that one (jdp asked
    # for "zwei Pfeile die einen Kreis bilden" there, and that file IS the
    # circle). Still a two-arrow loop, just the upright ring, so replicate and
    # recover stay tellable apart at 20px.
    ("IconSync", "interface-essential/arrow-reload-vertical-2.svg", "Replicate or synchronise"),
    ("IconGear", "interface-essential/cog.svg", "Settings"),
    ("IconClose", "interface-essential/delete-1.svg", "Close"),
    ("IconCopy", "interface-essential/copy-paste.svg", "Copy"),
    # The last five nav-rail glyphs. They were hand-drawn at 22x22 on a 20-unit
    # grid while everything generated here renders 16x16 on a 14-unit one, so
    # six rows of the rail sat at one size and six at another and the labels
    # beside them did not line up. Same source now, same box, one family.
    ("IconDashboard", "interface-essential/dashboard-3.svg", "Dashboard"),
    ("IconRecovery", "interface-essential/arrow-reload-horizontal-2.svg", "Recovery, rebuild from backups"),
    ("IconConfig", "computer-devices/database-setting.svg", "Configuration self-backup"),
    # The view toggle used to wear ONE glyph for both states, so the row looked
    # identical whichever view was on. Two now, and the pair carries the meaning
    # on its own: a sparse layout against a dense one.
    ("IconViewSimple", "interface-essential/layout-window-11.svg", "Simple view"),
    ("IconViewAdvanced", "interface-essential/layout-window-8.svg", "Advanced view"),
    # Two of Settings' seven tab glyphs. The other five are still hand-drawn in
    # Settings.tsx and stay that way; only these two were wrong, so only these
    # two moved. Keeping them here rather than drawing them there also keeps the
    # CC BY attribution in one file instead of two.
    ("IconTabSystem", "computer-devices/computer-chip-1.svg", "System tab"),
    # Back to Streamline's own after two hand-drawn attempts (jdp picked this
    # one by name). Keeping it in the generated list rather than as a drawing
    # also puts Flash back inside the one icon family.
    ("IconFlash", "computer-devices/usb-drive.svg", "The Unraid boot flash drive"),
]

# ---------------------------------------------------------------------------
# Hand-authored glyphs, emitted verbatim after the generated ones.
#
# One symbol cannot come from Streamline, and it would be lost on the next
# regeneration if it lived in the .tsx by hand — which is exactly what that
# file's own header forbids. So it lives here instead.
#
# (Flash was briefly here too, as a drawing, after the free set's own USB glyph
# was judged too plug-like. Two attempts later jdp picked Streamline's original
# by name, so it moved back into NAV. Worth remembering: drawing a replacement
# is the fallback, not the first move.)
#
# Each carries its own viewBox, so they do NOT go through `G` (which pins the
# 14-unit grid). The rendered box is the same 16px either way.
# ---------------------------------------------------------------------------
# The one cloud in the app. jdp picked this shape from a sheet and asked for it
# level; an arc chain has no straight edge to be level, so it is a rectangle
# plus three circles and the base is horizontal by construction. Defined once
# and emitted twice, because the Off-site TAB and every off-site control
# elsewhere must not drift apart again (jdp: "Das offsite-glyph systemweit an
# den glyph den Offsite einstellungstab angleichen").
CLOUD = (
    '<rect x="2.2" y="7.9" width="9.6" height="3.0" rx="1.5" />'
    '<circle cx="4.8" cy="7.2" r="2.6" />'
    '<circle cx="7.5" cy="5.9" r="3.2" />'
    '<circle cx="10.2" cy="7.5" r="2.4" />'
)

EXTRA_NAV = [
    (
        "IconContainers",
        "Docker containers",
        "0 0 24 24",
        # The Docker whale, from Simple Icons (simpleicons.org), which is CC0.
        # The MARK ITSELF is a trademark of Docker, Inc. and is used here the
        # one way a trademark may be used without permission: to refer to the
        # thing it names. This row navigates to Docker containers and nothing
        # else, the glyph is unmodified, and nothing about it claims
        # endorsement by or affiliation with Docker, Inc.
        '<path d="%s" />' % io.open("../scripts/docker-path.txt", encoding="utf-8").read().strip(),
    ),
    ("IconTabOffsite", "Off-site tab", "0 0 14 14", CLOUD),
    ("IconCloud", "Off-site or cloud", "0 0 14 14", CLOUD),
]

ATTRIBUTION = """// ---------------------------------------------------------------------------
// %s
//
// GENERATED by scripts/gen_glyphs.py. Do not hand-edit: regenerate instead, or
// the next run silently overwrites the change.
//
// Attribution (CC BY 4.0, required by the licence):
//   Free icons from Streamline - https://streamlinehq.com
//
// Only the FREE 1000-icon subset is used (github.com/webalys-hq/streamline-vectors,
// core/solid), which is CC BY 4.0 and explicitly redistributable. The larger
// 5771-icon set sold on streamlinehq.com is a different product whose licence
// forbids redistribution, which is exactly what a public repository does.
//
// Two changes are made on import, both load-bearing:
//   - fill becomes `currentColor`, so a glyph inherits the ink its button or
//     nav row paints and stays correct in every theme and in rainbow mode. The
//     source files hard-code #000000, which is invisible on a dark surface and
//     ignores the colour engine entirely.
//   - the source <desc> is dropped and `aria-hidden` added, because the
//     control's LABEL is the accessible name; a described glyph would be
//     announced on top of it.
//
// The source grid is 14 units. Everything renders into a 16px box regardless,
// so the viewBox differing from the box size is deliberate.
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
"""


# ---------------------------------------------------------------------------
# Optical sizing ([242]).
#
# What the eye compares is a glyph's INK, not its box. The cloud that means
# "off-site" is hand-drawn with air around it and covers 10.4 x 8.2 of the
# 14-unit grid. Streamline draws to the edge: hard-disk covers 12 x 14. In the
# identical 20px box that is roughly double the ink, so the drive read as a far
# bigger symbol beside the cloud it is supposed to pair with (jdp: "der glyph
# ist im vergleich zu offsite glyph viel zu gross").
#
# A fitted glyph is therefore scaled into the CLOUD's envelope and centred on
# it. Fitting by HEIGHT is what the min() below works out to here, and that is
# the right rule for a row of icons: the eye lines them up on their height, so
# a nearly-square drive ends up narrower than the wide cloud and still reads as
# the same size.
#
# The source boxes are MEASURED, via getBBox on the real markup in a browser,
# never derived from the viewBox — a path's drawn extent and its viewBox have
# no necessary relationship, and this whole bug was that assumption. Re-measure
# the same way if a source file is swapped; test glyphs.fit.test.ts pins the
# arithmetic so a regeneration cannot drift silently.
# ---------------------------------------------------------------------------
CLOUD_INK = (2.2, 2.7, 10.4, 8.2)  # IconCloud's own ink, measured

# glyph name -> the measured ink box of ITS source file
FIT_TO_CLOUD = {
    "IconLocal": (1.0, 0.0, 12.0, 14.0),  # computer-devices/hard-disk.svg
}


def num(v):
    """Shortest exact-enough decimal, so the generated file stays readable."""
    return ("%.6f" % v).rstrip("0").rstrip(".")


def fit_transform(name):
    """The `translate(...) scale(...)` that drops a glyph's ink inside the
    cloud's, or None when the glyph is already drawn at its natural size."""
    box = FIT_TO_CLOUD.get(name)
    if box is None:
        return None
    sx, sy, sw, sh = box
    tx, ty, tw, th = CLOUD_INK
    scale = min(tw / sw, th / sh)
    dx = (tx + tw / 2) - (sx + sw / 2) * scale
    dy = (ty + th / 2) - (sy + sh / 2) * scale
    return "translate(%s %s) scale(%s)" % (num(dx), num(dy), num(scale))


def body(path):
    raw = io.open(SRC + "/" + path, encoding="utf-8").read()
    raw = re.sub(r"<desc>.*?</desc>", "", raw, flags=re.S)
    inner = re.search(r"<svg[^>]*>(.*)</svg>", raw, re.S).group(1)
    inner = re.sub(r'\sid="[^"]*"', "", inner)
    inner = inner.replace('fill="#000000"', 'fill="currentColor"')
    inner = re.sub(r"\n\s*\n", "\n", inner).strip()
    for a, b in (
        ("fill-rule", "fillRule"),
        ("clip-rule", "clipRule"),
        ("stroke-width", "strokeWidth"),
        ("stroke-linecap", "strokeLinecap"),
        ("stroke-linejoin", "strokeLinejoin"),
        ("stroke-miterlimit", "strokeMiterlimit"),
    ):
        inner = inner.replace(a + "=", b + "=")
    # a single <g> wrapper adds nothing once the ids are gone
    m = re.match(r"^<g>\s*(.*?)\s*</g>$", inner, re.S)
    if m:
        inner = m.group(1)
    return "\n".join("      " + line.strip() for line in inner.split("\n"))


def write(path, headline, items, extra=()):
    out = [ATTRIBUTION % headline]
    for name, src, note in items:
        inner = body(src)
        transform = fit_transform(name)
        if transform:
            inner = '      <g transform="%s">\n%s\n      </g>' % (
                transform,
                "\n".join("  " + line for line in inner.split("\n")),
            )
        out.append(
            "\n/** %s. */\nexport function %s() {\n  return (\n    <G>\n%s\n    </G>\n  );\n}\n"
            % (note, name, inner)
        )
    for name, note, viewbox, markup in extra:
        out.append(
            "\n/** %s. */\nexport function %s() {\n  return (\n"
            '    <svg\n      width="16"\n      height="16"\n      viewBox="%s"\n'
            '      fill="currentColor"\n      className="shrink-0"\n      aria-hidden="true"\n    >\n'
            "      %s\n    </svg>\n  );\n}\n" % (note, name, viewbox, markup)
        )
    io.open(path, "w", encoding="utf-8", newline="").write("".join(out))
    print("wrote %d glyphs to %s (%d hand-authored)" % (len(items) + len(extra), path, len(extra)))


write(
    "src/components/glyphs.tsx",
    "Action glyphs (#178, [202]) - the verbs buttons wear.",
    ACTION,
)
write(
    "src/components/navGlyphs.tsx",
    "Navigation and domain glyphs (#178, [202]) - re-exported by Sidebar.tsx.",
    NAV,
    EXTRA_NAV,
)
