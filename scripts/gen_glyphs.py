"""Generate the app's glyphs from Streamline's free Core Solid set.

Run from web/:  python ../scripts/gen_glyphs.py <path-to-streamline-vectors/core/solid>

Source: github.com/webalys-hq/streamline-vectors, folder core/solid, CC BY 4.0.
That free 1000-icon subset may be redistributed under its licence terms. The
5771-icon set on streamlinehq.com is a separate premium product whose licence
forbids redistribution, so it cannot be used in a public repository.

Writes two files:
  src/components/glyphs.tsx      the action verbs buttons wear
  src/components/navGlyphs.tsx   the navigation and domain symbols

Sidebar.tsx re-exports navGlyphs, so IconTrash and the rest can also be
imported from there.
"""
import io
import re
import sys

SRC = sys.argv[1] if len(sys.argv) > 1 else "/tmp/slv/core/solid"

# our name -> (streamline file, what it means here)
ACTION = [
    # IconSave and IconCancel are in EXTRA_ACTION.
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
    # The About card's coffee and mail buttons.
    ("IconCoffee", "food-drink/coffee-takeaway-cup.svg", "Buy the author a coffee"),
    # An envelope rather than the set's send arrow: a mail address is somewhere
    # to write to, and sending is what the button beside it already means.
    ("IconMail", "mail/mail-send-envelope.svg", "Write to us"),
    # One door with the arrow turned around, so the pair reads as one. Not
    # IconForward and IconBack, which already mean wizard navigation.
    ("IconSignIn", "interface-essential/login-1.svg", "Sign in"),
    ("IconSignOut", "interface-essential/logout-1.svg", "Sign out"),
    # The second factor on and off. A shield, because the power mark already
    # means shutting a VM down; the cross sits inside it, so it cannot be read
    # as the bare Cancel cross.
    ("IconShieldOn", "interface-essential/shield-check.svg", "A protection is on"),
    ("IconShieldOff", "interface-essential/shield-cross.svg", "A protection is off"),
    # Two offset panes held against each other. The magnifier already means
    # scan and browse.
    ("IconCompare", "interface-essential/layers-2.svg", "Compare two things"),
]

# Glyphs that point along the reading direction, so a right-to-left layout
# mirrors them.
MIRRORED = {"IconBack", "IconForward", "IconSignIn", "IconSignOut"}

# Navigation and domain symbols, from the same set so the interface reads as
# one icon family.
NAV = [
    ("IconVM", "computer-devices/screen-1.svg", "Virtual machines"),
    ("IconFiles", "interface-essential/new-folder.svg", "Files and folder sets"),
    ("IconReceiver", "interface-essential/login-1.svg", "Receiver, an incoming transfer"),
    ("IconFleet", "interface-essential/hierarchy-2.svg", "Fleet, other BombVault boxes"),
    ("IconFolder", "interface-essential/new-folder.svg", "A folder"),
    # IconLocal, IconAdd and IconClose are in EXTRA_NAV.
    ("IconDownload", "interface-essential/download-box-1.svg", "Download or export"),
    ("IconBackupNow", "computer-devices/database-check.svg", "Back up now"),
    ("IconRestore", "interface-essential/arrow-reload-vertical-1.svg", "Restore"),
    ("IconPower", "entertainment/button-power-1.svg", "Power, start or stop"),
    ("IconLive", "interface-essential/live-video.svg", "Live, currently running"),
    ("IconTrash", "interface-essential/recycle-bin-2.svg", "Delete"),
    ("IconPencil", "interface-essential/pencil.svg", "Edit"),
    # IconCheckCircle and IconCopy are in EXTRA_NAV.
    # The upright loop, so that at 20px it stays distinct from IconRecovery's
    # horizontal one.
    ("IconSync", "interface-essential/arrow-reload-vertical-2.svg", "Replicate or synchronise"),
    ("IconGear", "interface-essential/cog.svg", "Settings"),
    ("IconDashboard", "interface-essential/dashboard-3.svg", "Dashboard"),
    ("IconRecovery", "interface-essential/arrow-reload-horizontal-2.svg", "Recovery, rebuild from backups"),
    ("IconConfig", "computer-devices/database-setting.svg", "Configuration self-backup"),
    # The view toggle: a sparse layout against a dense one.
    ("IconViewSimple", "interface-essential/layout-window-11.svg", "Simple view"),
    ("IconViewAdvanced", "interface-essential/layout-window-8.svg", "Advanced view"),
    # A Settings tab glyph from Streamline lives here rather than in
    # Settings.tsx, so the CC BY attribution stays in one file.
    ("IconTabSystem", "computer-devices/computer-chip-1.svg", "System tab"),
    ("IconFlash", "computer-devices/usb-drive.svg", "The Unraid boot flash drive"),
]


def num(v):
    """Shortest exact-enough decimal, so the generated file stays readable."""
    return ("%.6f" % v).rstrip("0").rstrip(".")


def cropped_box(ink):
    """A square viewBox tight to a glyph's measured ink, centred on it.

    Every glyph renders into the same 20px box, but imported sets pad their
    drawings differently: Font Awesome fills its box, Tabler leaves two units
    on every side. Cropping to the ink lets the default
    `preserveAspectRatio="xMidYMid meet"` scale each drawing until its longer
    side fills the box, so glyphs from any set and any grid come out the same
    size. The box is square because a tight rectangle would stretch a wide
    glyph to a tall one's height.

    `ink` is measured with getBBox on the real markup, never read off the
    viewBox: a path's drawn extent need not match its viewBox, and some source
    files carry a transparent path covering the whole of it.
    """
    x, y, w, h = ink
    side = max(w, h)
    return "%s %s %s %s" % (
        num(x + w / 2 - side / 2),
        num(y + h / 2 - side / 2),
        num(side),
        num(side),
    )


def imported(name, note, source_box, ink, path_file, even_odd=False):
    """One glyph imported whole from an outside set, cropped to its ink.

    `even_odd` keeps a source's evenodd fill rule. Where an outline and its
    cut-out wind the same way, as with the label panel of the save mark, the
    default nonzero rule would fill the hole.
    """
    paths = io.open(
        "../scripts/glyph-paths/%s.txt" % path_file, encoding="utf-8"
    ).read().strip().split("\n")
    del source_box  # kept in the call for reference; the crop replaces it
    # fillRule, not fill-rule: this lands in JSX, and body() renames the SVG
    # attributes only for Streamline imports.
    rule = ' fillRule="evenodd"' if even_odd else ""
    return (name, note, cropped_box(ink), "".join('<path%s d="%s" />' % (rule, d) for d in paths))


# Shared by the Off-site tab and every other off-site control so the two cannot
# drift apart. At 20px one grid unit is 1.43px and thinner detail merges with
# its neighbour, so the cloud is Font Awesome Free's `cloud`: one closed path
# with deep valleys and nothing small enough to disappear.
CLOUD_BOX = cropped_box((0.0, 32.0, 640.0, 448.0))
CLOUD = '<path d="%s" />' % io.open("../scripts/cloud-path.txt", encoding="utf-8").read().strip()

# Streamline's add-1 spans the whole 14-unit grid with thin arms and reads
# oversized and weak at once, so the plus has 10-unit arms and 2.8-unit bars.
_PLUS_BARS = (
    '<rect x="2" y="5.6" width="10" height="2.8" rx="1.4" />'
    '<rect x="5.6" y="2" width="2.8" height="10" rx="1.4" />'
)

# The cross is the plus turned 45 degrees about the grid centre, so the two
# stay a matched pair. It needs its own bars and frame because the eye reads
# size as reach, the distance from the centre to the furthest ink: a cross puts
# its tips on the corners of its box, a round glyph puts its ink on the edge
# midpoints, so in the same box the cross reaches sqrt(2) further.
#
# The bars are 2.2 so the painted area matches the neighbouring glyph's
# 145.4px^2: 20t - 1.4292t^2 = 36.35 units^2. That puts them at 22% of the box,
# close to the plus's 20%.
_CROSS_BARS = (
    '<rect x="2" y="5.9" width="10" height="2.2" rx="1.1" />'
    '<rect x="5.9" y="2" width="2.2" height="10" rx="1.1" />'
)
PLUS = _PLUS_BARS
CROSS = '<g transform="rotate(45 7 7)">%s</g>' % _CROSS_BARS

# The cross frame is 10 units centred on (7, 7), the rotation centre, so only
# the frame moves and not the ink. The tips (radius 5) land at 10px on a 20px
# render, where a glyph filling the 14-unit grid puts its outermost ink, and
# the turned cross measures 8.6 units, inside the frame; a smaller frame clips
# the tips.
#
# The plus keeps the full grid. It points at the edge midpoints like a circle,
# so it needs no correction.
#
# On the Fleet card the cross reaches 10.03px against the reload glyph's
# 10.96px. Leave that gap: the reload glyph exceeds 10px only because one
# arrowhead runs into a corner, and matching it would take a 9.12 box that
# makes the cross look oversized beside the rest of the set.
PLUS_BOX = "0 0 14 14"
CROSS_BOX = "2 2 10 10"

# The database cylinder. Every database glyph in Streamline's free set carries
# a second mark (a check, a cog, a cross) and both of those already mean
# something else here, so this one is drawn on the same 14-unit grid: a cap and
# two bands, each gap as wide at the centre as at the sides, which is what makes
# a single-colour stack read as a stack.
DATABASE = (
    '<ellipse cx="7" cy="3.3" rx="5.4" ry="1.8" />'
    '<path d="M1.6 5.7Q7 9.3 12.4 5.7L12.4 7.7Q7 11.3 1.6 7.7Z" />'
    '<path d="M1.6 8.7Q7 12.3 12.4 8.7L12.4 10.7Q7 14.3 1.6 10.7Z" />'
)
DATABASE_BOX = "0 0 14 14"

# ZFS datasets. Three separated platters on the same 14-unit grid as the
# database cylinder: the pool is a stack of disks, and keeping the gaps open
# stops the two marks reading as the same object in the rail.
ZFS = (
    '<ellipse cx="7" cy="2.6" rx="5.4" ry="1.8" />'
    '<ellipse cx="7" cy="7" rx="5.4" ry="1.8" />'
    '<ellipse cx="7" cy="11.4" rx="5.4" ry="1.8" />'
)
ZFS_BOX = "0 0 14 14"

# Multi-line notes become block comments in the generated files; see doc().
CLOSE_NOTE = """Close

IconAdd's plus turned 45 degrees, in its own frame. A cross puts its tips on
the corners of its box, so in the full grid it would reach further than its
neighbours and look larger. The 10-unit frame, centred on the rotation
centre (7, 7), puts the tips where a glyph filling the 14-unit grid puts its
outermost ink. getBBox reports a rotated group's extent before rotation, so
measure this glyph rasterised."""

CANCEL_NOTE = """Cancel or dismiss

The same drawing and cropped viewBox as navGlyphs' IconClose, which explains
the frame."""

# Glyphs that do not come from the Streamline set. They are emitted after the
# generated ones and carry their own viewBox instead of going through G's
# 14-unit grid; the rendered box is 16px either way.
EXTRA_NAV = [
    (
        "IconContainers",
        "Docker containers",
        "0 0 24 24",
        # The Docker whale from Simple Icons (CC0). The mark is a trademark of
        # Docker, Inc., used unmodified only to name what the row leads to,
        # with no claim of endorsement or affiliation.
        '<path d="%s" />' % io.open("../scripts/docker-path.txt", encoding="utf-8").read().strip(),
    ),
    ("IconTabOffsite", "Off-site tab", CLOUD_BOX, CLOUD),
    ("IconCloud", "Off-site or cloud", CLOUD_BOX, CLOUD),
    ("IconDatabase", "A database", DATABASE_BOX, DATABASE),
    ("IconZFS", "ZFS datasets", ZFS_BOX, ZFS),
    ("IconAdd", "Add", PLUS_BOX, PLUS),
    ("IconClose", CLOSE_NOTE, CROSS_BOX, CROSS),
    # Imported whole and cropped to their measured ink. Sources and licences,
    # all attributed in ATTRIBUTION below:
    #   save                  - Vecteezy, Free License (attribution required)
    #   storage, local        - Font Awesome Free (CC BY 4.0)
    #   copy                  - Tabler Icons, filled variant (MIT)
    #   integrity             - Material Design Icons (Apache 2.0)
    #   verify                - shipped as an Illustrator export
    #
    # copy and verify each came with a transparent path covering the whole
    # viewBox. It is dropped on import: it paints nothing and would make every
    # ink measurement read 100%.
    imported("IconLocal", "Local storage, as opposed to off-site",
             "0 0 512 512", (0.0, 32.0, 512.0, 448.0), "local"),
    imported("IconCopy", "Copy", "0 0 24 24", (2.0, 2.0, 20.0, 20.0), "copy"),
    # Kept at the precision getBBox reported, not rounded to 2, 2, 20, 20.
    imported("IconCheckCircle", "Verify, check or test a connection",
             "0 0 24 24", (2.0, 1.9934, 20.0078, 20.0143), "verify"),
    imported("IconTabIntegrity", "Integrity tab", "0 0 24 24",
             (3.0, 1.0, 18.0, 22.0), "integrity"),
    imported("IconTabStorage", "Paths and storage tab", "0 0 448 512",
             (0.0, 0.0, 448.0, 512.0), "storage"),
]

# IconCancel is the same cross as IconClose, so the app has one X.
EXTRA_ACTION = [
    ("IconCancel", CANCEL_NOTE, CROSS_BOX, CROSS),
    # Vecteezy's floppy, the same file ArrowLoop uses. Its Free License asks for
    # the name and a link, both in ATTRIBUTION. The source box is 0 0 492 492
    # but the drawing fills only a 368.7 square of it; ArrowLoop's
    # scripts/measure_ink.py measures that without a browser.
    imported("IconSave", "Save", "0 0 492 492", (61.80, 62.40, 368.70, 368.70), "save",
             even_odd=True),
    # A brand mark like the Docker whale, also from Simple Icons (CC0), and
    # used on the same trademark terms. Every other glyph stands for what a
    # button does, the same shape wherever the action appears; a logo names
    # one company, so it belongs only where the control leads to that
    # company's own thing: the repository button on the About card and the
    # Docker row in the rail.
    #
    # Brand marks are never wired into glyphFor, where a rule keyed on "repo"
    # would put GitHub's logo on repository settings that have nothing to do
    # with GitHub. They are passed as an explicit glyph at the one call site
    # that means them.
    imported("IconGithub", "The project's GitHub repository", "0 0 24 24", (0.0, 0.297, 24.0, 23.406), "github"),
]

ATTRIBUTION = """// %s
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
//   IconSave from Vecteezy - https://www.vecteezy.com
//
// Only Streamline's free 1000-icon subset is used
// (github.com/webalys-hq/streamline-vectors, core/solid), which is CC BY 4.0
// and may be redistributed. The 5771-icon set sold on streamlinehq.com is a
// separate product whose licence forbids redistribution.
//
// Vecteezy's Free License asks for Vecteezy.com to be credited in the design,
// with a link to vecteezy.com where possible; the line above does that. Font
// Awesome Free puts its icons under CC BY 4.0 (its fonts are SIL OFL, its code
// MIT), and the off-site cloud (scripts/cloud-path.txt) is one of those icons,
// so the attribution above covers it.
//
// On import, fill becomes `currentColor` so a glyph takes the ink of its
// control in every theme, and the source <desc> gives way to `aria-hidden`,
// because the control's label is its accessible name.
//
// The source grid is 14 units; every glyph renders into a 16px box.

import type { ReactNode } from "react";

function G({ children, mirror = false }: { children: ReactNode; mirror?: boolean }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 14 14"
      fill="currentColor"
      className={mirror ? "shrink-0 rtl:-scale-x-100" : "shrink-0"}
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


def doc(note):
    """The doc comment above one glyph.

    A one-line note becomes `/** Refresh or reload. */`. A note with newlines
    becomes a block comment whose first line is the summary, so a glyph's
    reasoning can live in this generator instead of being hand-written into
    files that every run overwrites.
    """
    lines = note.split("\n")
    if len(lines) == 1:
        return "/** %s. */" % note
    out = ["/**", " * %s." % lines[0]]
    for line in lines[1:]:
        out.append(" *" if not line else " * %s" % line)
    out.append(" */")
    return "\n".join(out)


def write(path, headline, items, extra=()):
    out = [ATTRIBUTION % headline]
    for name, src, note in items:
        out.append(
            "\n%s\nexport function %s() {\n  return (\n    <G%s>\n%s\n    </G>\n  );\n}\n"
            % (doc(note), name, " mirror" if name in MIRRORED else "", body(src))
        )
    for name, note, viewbox, markup in extra:
        out.append(
            "\n%s\nexport function %s() {\n  return (\n"
            '    <svg\n      width="16"\n      height="16"\n      viewBox="%s"\n'
            '      fill="currentColor"\n      className="shrink-0"\n      aria-hidden="true"\n    >\n'
            "      %s\n    </svg>\n  );\n}\n" % (doc(note), name, viewbox, markup)
        )
    io.open(path, "w", encoding="utf-8", newline="").write("".join(out))
    print("wrote %d glyphs to %s (%d hand-authored)" % (len(items) + len(extra), path, len(extra)))


write(
    "src/components/glyphs.tsx",
    "Action glyphs, the verbs buttons wear.",
    ACTION,
    EXTRA_ACTION,
)
write(
    "src/components/navGlyphs.tsx",
    "Navigation and domain glyphs, re-exported by Sidebar.tsx.",
    NAV,
    EXTRA_NAV,
)
