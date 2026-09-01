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
    # IconSave moved to EXTRA_ACTION ([316]) — jdp's own file.
    # IconCancel moved to EXTRA_ACTION ([287]) — same hand-drawn cross as
    # IconClose, so the app has one X rather than two.
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
    # detail finer than the raster could hold ([281]). Worth remembering in
    # both directions — [242] moved a glyph OUT of a drawing and back to
    # Streamline's own, this one moved the other way, and each time the
    # deciding evidence was the same: what survives at 20px.
    # IconAdd and IconClose moved to EXTRA_NAV ([287]) — hand-drawn, shorter
    # arms and thicker bars than Streamline's full-grid ones.
    ("IconDownload", "interface-essential/download-box-1.svg", "Download or export"),
    ("IconBackupNow", "computer-devices/database-check.svg", "Back up now"),
    ("IconRestore", "interface-essential/arrow-reload-vertical-1.svg", "Restore"),
    ("IconPower", "entertainment/button-power-1.svg", "Power, start or stop"),
    ("IconLive", "interface-essential/live-video.svg", "Live, currently running"),
    ("IconTrash", "interface-essential/recycle-bin-2.svg", "Delete"),
    ("IconPencil", "interface-essential/pencil.svg", "Edit"),
    # IconCheckCircle moved to EXTRA_NAV ([318]) — jdp's own file, and it now
    # covers every check/test button rather than only "verified".
    # Moved off arrow-reload-horizontal-2 when Recovery took that one (jdp asked
    # for "zwei Pfeile die einen Kreis bilden" there, and that file IS the
    # circle). Still a two-arrow loop, just the upright ring, so replicate and
    # recover stay tellable apart at 20px.
    ("IconSync", "interface-essential/arrow-reload-vertical-2.svg", "Replicate or synchronise"),
    ("IconGear", "interface-essential/cog.svg", "Settings"),
    # IconCopy moved to EXTRA_NAV ([319]) — jdp's own file.
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
# The Local / Off-site pair ([281]).
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
def num(v):
    """Shortest exact-enough decimal, so the generated file stays readable."""
    return ("%.6f" % v).rstrip("0").rstrip(".")


def cropped_box(ink):
    """A square viewBox tight to a glyph's measured INK, centred on it.

    This is the one sizing mechanism ([316]-[321] unified what used to be two).
    Everything in this set renders into the same 20px box, so what decides
    whether two icons look the same size is how much of that box each one's
    drawing actually uses — and imported artwork varies wildly: Font Awesome
    fills its box edge to edge, Tabler leaves two units of padding on all four
    sides, a hand-drawn glyph leaves whatever it was drawn with. Measured
    across one screen that spread ran from 68% to 100%, and it read exactly as
    "some of these icons are smaller".

    Cropping the viewBox to the ink and squaring it off hands the whole
    problem to `preserveAspectRatio="xMidYMid meet"`, which is already the
    default: the drawing scales up until its longer side fills the box, and
    its aspect ratio is untouched. Square rather than tight-on-both-axes for
    exactly that reason — a tight rectangle would stretch a wide glyph to the
    same height as a tall one.

    Why this replaced the translate/scale transform the off-site pair used for
    two rounds: a transform has to know the target grid, so it only works for
    glyphs already living on that grid, and it needed a shared PAIR_WIDTH
    constant to keep two of them agreeing. A cropped viewBox needs neither. It
    works on a 24-unit Tabler icon and a 512-unit Font Awesome one without
    either being converted first, and two glyphs agree because they follow the
    same rule rather than because they share a number.

    `ink` is always MEASURED, with getBBox on the real markup in a browser,
    never read off the viewBox — a path's drawn extent and its viewBox have no
    necessary relationship, and two of jdp's own files carry a fully
    transparent bounding path that makes the viewBox actively misleading.
    """
    x, y, w, h = ink
    side = max(w, h)
    return "%s %s %s %s" % (
        num(x + w / 2 - side / 2),
        num(y + h / 2 - side / 2),
        num(side),
        num(side),
    )


def imported(name, note, source_box, ink, path_file):
    """One glyph imported whole from an outside set, cropped to its ink."""
    paths = io.open(
        "../scripts/glyph-paths/%s.txt" % path_file, encoding="utf-8"
    ).read().strip().split("\n")
    del source_box  # kept in the call for the record; the crop supersedes it
    return (name, note, cropped_box(ink), "".join('<path d="%s" />' % d for d in paths))


# Defined once and emitted twice, because the Off-site TAB and every off-site
# control elsewhere must not drift apart again (jdp: "Das offsite-glyph
# systemweit an den glyph den Offsite einstellungstab angleichen").
CLOUD_BOX = cropped_box((0.0, 32.0, 640.0, 448.0))
CLOUD = '<path d="%s" />' % io.open("../scripts/cloud-path.txt", encoding="utf-8").read().strip()

# ---------------------------------------------------------------------------
# The plus and the cross ([287]).
#
# jdp: "auf den schließen und hinzufügen buttons ist das plus und x zu groß.
# können wir das ein glyph mit breiteren strichen verwenden?"
#
# Both are right and they are the same observation. Streamline's `add-1` and
# `delete-1` are drawn as thin arms spanning the FULL 14-unit grid, so measured
# in the app they filled 14x14 of 14 — 20px of drawn mark in a 20px box, edge
# to edge. Every other glyph in the app is a recognisable object that reads at a
# glance; a plus stretched to the same extent is just two long thin lines, and
# thin plus long is exactly the combination that looks oversized and weak at the
# same time.
#
# So: 10 units of arm instead of 14, and 2.8 units of bar instead of the
# source's ~2. Shorter and thicker, which is what "breitere Striche" asks for.
#
# The cross is the plus turned 45 degrees around the grid's centre rather than a
# second drawing. Two marks that are meant to read as a matched pair cannot
# drift apart if there is only one of them, and the rotation is exact where
# hand-placed diagonals would each need their own corner arithmetic.
_PLUS_BARS = (
    '<rect x="2" y="5.6" width="10" height="2.8" rx="1.4" />'
    '<rect x="5.6" y="2" width="2.8" height="10" rx="1.4" />'
)

# The cross is the plus rotated, and a rotated mark needs its own frame AND its
# own bar. Three rounds got here, and each one measured a different quantity:
#
#   [426] measured EXTENT. The turned cross filled 58% of its box where 45 of 48
#         glyphs filled 100%, so it read as the smallest mark in the set. Fixed
#         by cropping the box to 8.12.
#   [514] measured AREA. The crop had left 2.8-unit bars against a box that went
#         14 -> 8.12, turning a stroke that was a fifth of its glyph into a
#         third. jdp: "wirkt viel zu klobig". Bars thinned to 1.0, which put the
#         painted area at 37.7% against the neighbour's 36.6%.
#   [526] measured REACH, and that is the one that was wrong all along. jdp,
#         with the areas matched: "das X Glyph ist zu groß."
#
# REACH is the distance from the centre to the furthest ink, and it is what the
# eye calls "size" for marks of different shape. A CROSS PUTS ITS FOUR TIPS ON
# THE CORNERS OF ITS BOUNDING BOX; a circular glyph puts its ink on the edge
# midpoints. Fill the same box with both and the cross reaches sqrt(2) further —
# 14.1px against 10px at a 20px render. Area can match exactly while that holds,
# because the cross spends its ink thinly along four diagonals and the loop
# packs the same ink into a ring. Both of my earlier measurements were true and
# neither was the complaint.
#
# The general lesson, which cost three rounds: A MEASUREMENT THAT AGREES WITH
# ITSELF IS NOT A FINDING. Extent, area and reach are three different questions,
# and matching two of them says nothing about the third.
#
# So: tip radius 5 units in a 10-unit box = 10px at a 20px render, exactly the
# neighbour's radius. Bars back up to 2.2 because the glyph itself shrank by
# 1/sqrt(2) and area goes with the square of that; 2.2 solves
# 20t - 1.4292t^2 = 36.35 units^2, which is the neighbour's 145.4px^2. That
# also lands the bar at 22% of its box, within a whisker of the plus's 20%, so
# the two marks read as the siblings they are drawn from.
_CROSS_BARS = (
    '<rect x="2" y="5.9" width="10" height="2.2" rx="1.1" />'
    '<rect x="5.9" y="2" width="2.2" height="10" rx="1.1" />'
)
PLUS = _PLUS_BARS
CROSS = '<g transform="rotate(45 7 7)">%s</g>' % _CROSS_BARS

# The cross needs its OWN box — [426] was right about that and wrong about the
# number, see the bars above.
#
# 10 units, centred on (7,7): the same point the rotation turns about, so the
# ink does not move and only the frame around it does. That puts the arm tips
# (radius 5 units) at 10px on a 20px render, which is exactly where a glyph that
# fills the 14-unit grid puts its outermost ink. The bounding box of the turned
# cross then measures 8.6 units, comfortably inside the frame — the 7.1 box
# before this was narrower than its own drawing and CLIPPED all four tips
# (measured: 21.9px of ink in a 20px frame).
#
# The plus keeps the full 14-unit grid and reads at 72% of it. That is a real
# difference from the cross's 86%, and it is deliberate: the plus points at the
# edge midpoints where a circle also sits, so it needs no correction, while the
# cross points at the corners and does. Give both the same frame and one of them
# is wrong; this is which.
#
# LEAVE THE 8% GAP TO THE RELOAD GLYPH ALONE. Measured live on the Fleet card,
# where the two sit side by side: the cross reaches 10.03px and its neighbour
# 10.96px at a 20px render. The neighbour is over 10 because ONE arrowhead runs
# into a corner, which is a property of that drawing and not of the set — a
# frame-filling circle measures 10.02. Tuning the cross to 10.96 would need a
# 9.12 box and would then read oversized beside the other 46 glyphs, which is
# the complaint this whole line of work started from. A diagonal mark sitting a
# little inside the circle keyline is also what icon sets do on purpose.
PLUS_BOX = "0 0 14 14"
CROSS_BOX = "2 2 10 10"

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
    ("IconTabOffsite", "Off-site tab", CLOUD_BOX, CLOUD),
    ("IconCloud", "Off-site or cloud", CLOUD_BOX, CLOUD),
    ("IconAdd", "Add", PLUS_BOX, PLUS),
    ("IconClose", "Close", CROSS_BOX, CROSS),
    # ---------------------------------------------------------------------
    # jdp's own files ([316]-[321]), each cropped to its measured ink.
    #
    # Six drawings from four different sets, which is precisely why the crop
    # exists: Font Awesome fills its box, Tabler pads it by two units on every
    # side, and Material sits somewhere between. Left alone they would have
    # arrived on screen at three different sizes.
    #
    # Sources and licences, all attributed in the header above:
    #   save, storage, local  - Font Awesome Free (CC BY 4.0)
    #   copy                  - Tabler Icons, filled variant (MIT)
    #   integrity             - Material Design Icons (Apache 2.0)
    #   verify                - shipped as an Illustrator export
    #
    # `copy` and `verify` each arrived with a fully transparent bounding path
    # covering the whole viewBox. Those are dropped on import: they paint
    # nothing, and left in place they make every ink measurement read 100%.
    # ---------------------------------------------------------------------
    imported("IconLocal", "Local storage, as opposed to off-site",
             "0 0 512 512", (0.0, 32.0, 512.0, 448.0), "local"),
    imported("IconCopy", "Copy", "0 0 24 24", (2.0, 2.0, 20.0, 20.0), "copy"),
    # Re-measured after jdp revised the drawing. The envelope barely moved
    # (2, 1.9934, 20.0078, 20.0143 against the previous 2, 2, 20, 20) and the
    # numbers are carried at full precision anyway: the declared ink has to be
    # the MEASURED ink, or the discipline the whole crop rests on is already
    # gone, and the next re-measure has nothing trustworthy to compare against.
    imported("IconCheckCircle", "Verify, check or test a connection",
             "0 0 24 24", (2.0, 1.9934, 20.0078, 20.0143), "verify"),
    imported("IconTabIntegrity", "Integrity tab", "0 0 24 24",
             (3.0, 1.0, 18.0, 22.0), "integrity"),
    imported("IconTabStorage", "Paths and storage tab", "0 0 448 512",
             (0.0, 0.0, 448.0, 512.0), "storage"),
]

# Same two marks for the ACTION set, from the same constants. IconCancel is the
# same X as IconClose and used to be the same Streamline file; leaving it on the
# import while its twin moved would have put two different crosses on one
# screen.
EXTRA_ACTION = [
    ("IconCancel", "Cancel or dismiss", CROSS_BOX, CROSS),
    # jdp's save glyph ([316]), replacing Streamline's floppy-disk everywhere a
    # button means "save".
    imported("IconSave", "Save", "0 0 448 512", (0.0, 32.0, 448.0, 448.0), "save"),
]

ATTRIBUTION = """// ---------------------------------------------------------------------------
// %s
//
// GENERATED by scripts/gen_glyphs.py. Do not hand-edit: regenerate instead, or
// the next run silently overwrites the change.
//
// Attribution, required by the licences below:
//   Free icons from Streamline - https://streamlinehq.com (CC BY 4.0)
//   Font Awesome Free - https://fontawesome.com (icons: CC BY 4.0)
//   Tabler Icons - https://tabler.io/icons (MIT)
//   Material Design Icons - https://pictogrammers.com/library/mdi/ (Apache 2.0)
//   Simple Icons - https://simpleicons.org (CC0)
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
    EXTRA_ACTION,
)
write(
    "src/components/navGlyphs.tsx",
    "Navigation and domain glyphs (#178, [202]) - re-exported by Sidebar.tsx.",
    NAV,
    EXTRA_NAV,
)
