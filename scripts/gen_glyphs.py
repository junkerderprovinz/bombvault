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
    # ("IconLocal", ...) used to live here, as a folder, then hard-drive-1,
    # then hard-disk. It is hand-drawn now and sits with the cloud in
    # EXTRA_NAV below: at 20px every imported drive glyph carried interior
    # detail finer than the raster could hold ([267]). Worth remembering in
    # both directions — [242] moved a glyph OUT of a drawing and back to
    # Streamline's own, this one moved the other way, and each time the
    # deciding evidence was the same: what survives at 20px.
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
# ---------------------------------------------------------------------------
# The Local / Off-site pair ([267]).
#
# jdp, looking at the deployed switch: "das offsite icon ist wenn es so klein
# ist sehr schlecht erkennbar, es muss einfacher sein." What breaks at 20px is
# not overall size but FEATURE size. A 14-unit grid rendered into 20px makes
# one unit 1.43px, so any detail thinner than roughly 1.5 units merges into its
# neighbour. The cloud drawn before this was three overlapping circles whose
# humps rose only ~1.3 units above the body: under the threshold, so they
# merged and the silhouette read as a plain dome. Streamline's hard-disk failed
# the same test from the other side, its interior arm being well under a unit
# wide.
#
# So both halves are now built from few, large shapes:
#
#   - the cloud is jdp's own file, Font Awesome 5 Free's `cloud` (see the
#     attribution block below). One closed path, deep valleys, nothing small
#     enough to disappear.
#   - "local" is two rounded bars and nothing else. It was picked over a disc
#     and a slotted block precisely because it has no interior detail at all.
#
# WHY THEY ARE MATCHED ON WIDTH, where the round before this matched on height:
# that earlier rule was for a nearly-square drive standing beside a wide cloud,
# where equal height is what makes a row look level. These two are both wide
# and their aspect ratios are close (1.43 against 1.60), so width is the shared
# dimension; matching heights instead would leave the cloud a full unit
# narrower than the bars and read as the smaller symbol. Both are centred on
# (7, 7) and span 9.6 units, which is the width the bars are drawn at.
#
# Measured with getBBox on the real markup, never inferred from a viewBox:
# the cloud's ink is (0, 32, 640, 448) inside its own 640x512 box, the bars'
# is (2.2, 4, 9.6, 6). navGlyphs.fit.dom.test.tsx recomputes the transform from
# those numbers, so a swapped path file fails instead of silently resizing.
# ---------------------------------------------------------------------------
PAIR_WIDTH = 9.6
PAIR_CENTRE = (7.0, 7.0)

# Ink of Font Awesome's cloud path, measured inside its 640x512 viewBox.
FA_CLOUD_INK = (0.0, 32.0, 640.0, 448.0)


def num(v):
    """Shortest exact-enough decimal, so the generated file stays readable."""
    return ("%.6f" % v).rstrip("0").rstrip(".")


def fit_to_pair(ink):
    """`translate(...) scale(...)` mapping a measured ink box onto the pair's
    shared width, centred on the pair's centre."""
    sx, sy, sw, sh = ink
    cx, cy = PAIR_CENTRE
    scale = PAIR_WIDTH / sw
    return "translate(%s %s) scale(%s)" % (
        num(cx - (sx + sw / 2) * scale),
        num(cy - (sy + sh / 2) * scale),
        num(scale),
    )


# Defined once and emitted twice, because the Off-site TAB and every off-site
# control elsewhere must not drift apart again (jdp: "Das offsite-glyph
# systemweit an den glyph den Offsite einstellungstab angleichen").
CLOUD = '<g transform="%s"><path d="%s" /></g>' % (
    fit_to_pair(FA_CLOUD_INK),
    io.open("../scripts/cloud-path.txt", encoding="utf-8").read().strip(),
)

# "Local", as opposed to off-site. Drawn rather than imported: no free set has
# a storage glyph this bare, and bareness is the whole requirement. Two bars,
# no LEDs, no slot, no hole.
LOCAL_DRIVE = (
    '<rect x="2.2" y="4" width="9.6" height="2.6" rx="1.3" />'
    '<rect x="2.2" y="7.4" width="9.6" height="2.6" rx="1.3" />'
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
    ("IconLocal", "Local storage, as opposed to off-site", "0 0 14 14", LOCAL_DRIVE),
]

ATTRIBUTION = """// ---------------------------------------------------------------------------
// %s
//
// GENERATED by scripts/gen_glyphs.py. Do not hand-edit: regenerate instead, or
// the next run silently overwrites the change.
//
// Attribution (CC BY 4.0, required by both licences):
//   Free icons from Streamline - https://streamlinehq.com
//   Font Awesome Free - https://fontawesome.com (icons: CC BY 4.0)
//
// Only the FREE 1000-icon subset is used (github.com/webalys-hq/streamline-vectors,
// core/solid), which is CC BY 4.0 and explicitly redistributable. The larger
// 5771-icon set sold on streamlinehq.com is a different product whose licence
// forbids redistribution, which is exactly what a public repository does.
//
// One glyph comes from Font Awesome Free instead: the off-site cloud
// (scripts/cloud-path.txt, their `cloud` solid). Font Awesome Free splits its
// licence by asset type, and only the ICONS are CC BY 4.0 - the fonts are SIL
// OFL and the code is MIT. A single path counts as an icon, so attribution is
// the whole obligation here, and it is discharged by the line above plus this
// note.
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
        out.append(
            "\n/** %s. */\nexport function %s() {\n  return (\n    <G>\n%s\n    </G>\n  );\n}\n"
            % (note, name, body(src))
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
