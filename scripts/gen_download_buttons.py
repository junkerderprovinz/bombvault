"""Generate the README's download buttons from the list in download_buttons.py.

This file is the same in every repository with download buttons; only
download_buttons.py beside it differs. It writes one SVG per button, a sprite
holding them together with the donation buttons, and every button row in
README.md.

The buttons come in the same five groups, in this order: the server (the
Unraid template, the container, the source and the manual), the desktop apps,
the phone apps, the browser extensions and the house's companion apps. What a
repository does not ship is left out. A group fills a row of four places
before it starts the next, and a blank line stands between two groups, so the
gap between groups is about twice the gap between rows.
Windows on ARM and the portable build are segments of the Windows button, and
Linux on ARM one of the Linux button, so the desktop row keeps to the four
places a row has.

Size and corner radius are the Buy Me a Coffee button's (841.9 by 245.3, rx
38.2), so every button on the page has the same shape.

The logos are the platforms' own marks from Font Awesome Free (CC BY 4.0 for the
icons), Vivaldi's and F-Droid's from Simple Icons, and Unraid's from Dashboard
Icons (Apache-2.0); see scripts/brand-paths/. The companions carry their own
logos. Every mark is drawn in the button's one ink.
Each is a trademark of its owner, used unmodified and only to name the platform
a button downloads for, with no claim of endorsement by or affiliation with its
owner. The ZIP and the book are Font Awesome's file-zipper and book, nobody's
mark.

Run from anywhere:  uv run --no-project --python 3.12 python scripts/gen_download_buttons.py
Writes .github/assets/download-buttons/*.svg, which are committed, and the
button rows in README.md between their markers, all of them showing one
sprite, .github/assets/download-buttons/buttons.svg.
"""

import glob
import hashlib
import http.client
import io
import math
import os
import re
import sys
import time
import urllib.error
import urllib.request
from html import escape
from types import SimpleNamespace

# Importing the button list would otherwise leave a __pycache__ in scripts/.
sys.dont_write_bytecode = True
import download_buttons as config

HERE = os.path.dirname(os.path.abspath(__file__))
# Relative to this file, so the generator works from any working directory.
OUT = os.path.join(HERE, "..", ".github", "assets", "download-buttons")
BRANDS = os.path.join(HERE, "brand-paths")

W, H, R = 841.9, 245.3, 38.2

# The mark is drawn into a square this tall, centred vertically, inset from the
# left. Its own viewBox decides the horizontal centring, because the marks are
# not equally wide: Apple's is 384 units against Windows' and Tux's 448. The
# inset keeps the mark near the left edge and leaves a clear gap before the
# words at TEXT_X.
GLYPH = 132.0
GX, GY = 85.0, (H - GLYPH) / 2
TEXT_X = 300
# Docker's whale, 640 units wide against 512 high, is as wide as a mark gets
# before it runs into the words. A wider one is scaled to this width instead.
WIDEST = GLYPH * 1.25
# A mark drawn as several shapes gets an outline this wide in its ink, since
# shapes that only touch leave a seam where each edge is anti-aliased alone.
SEAM = 1.0
# ParleyPort's flag leaves much of its box empty and reads small beside the
# solid marks, so it is drawn a little larger.
LARGER = {"parleyport": 1.12}

# A system stack, because an SVG loaded through <img> cannot fetch a webfont.
# The layout leaves room for a face wider than the one it was measured with.
FONT = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif"

COMPANIONS = 4
# Every button a repository can have, in the order the rows show them: group,
# brand file, background, ink, heading, second line, accessible name. The same
# thing carries the same words in every README.
WINDOWS = ("#0078d4", "#ffffff")
KINDS = {
    # The middle of the orange in Unraid's logo, where white holds 3.4:1,
    # enough for type this large.
    "unraid":           (0, "unraid", "#f15a2c", "#ffffff", "Unraid", "Template", "Install from Unraid's Community Applications"),
    "docker":           (0, "docker", "#1d63ed", "#ffffff", "Docker", "Container", "Run it with Docker"),
    "compose":          (0, "docker", "#1d63ed", "#ffffff", "Docker", "compose file", "Download the docker-compose file"),
    # Slate, since GitHub's black vanishes in the dark theme.
    "source":           (0, "zip", "#4d5562", "#ffffff", "Source", "zip archive", "Download the source archive"),
    # "Docs" rather than "Documentation": 13 characters at font-size 82 need
    # more than the 542 units left of the right edge. The yellow is the coffee
    # button's #fd0, and white on yellow fails contrast, so the ink is dark.
    "docs":             (0, "book", "#fd0", "#0d0c23", "Docs", "online manual", "Read the documentation"),
    "windows":          (1, "windows", *WINDOWS, "Windows", "x64", "Download for Windows"),
    "windows-arm":      (1, None, *WINDOWS, "Windows", "ARM64", "Download for Windows on ARM"),
    "windows-portable": (1, None, *WINDOWS, "Windows", "Portable", "Download the portable Windows app"),
    "windows-script":   (1, "windows", *WINDOWS, "Windows", "start script", "Download the Windows start script"),
    # Space grey, since black vanishes against GitHub's dark theme.
    "macos":            (1, "apple", "#6e6e73", "#ffffff", "macOS", "Universal", "Download for macOS"),
    # Tux yellow, with dark ink for contrast.
    "linux":            (1, "linux", "#fcc624", "#1b1b1b", "Linux", "x64", "Download for Linux"),
    "linux-arm":        (1, None, "#fcc624", "#1b1b1b", "Linux", "ARM64", "Download for Linux on ARM"),
    "linux-script":     (1, "linux", "#fcc624", "#1b1b1b", "Linux", "start script", "Download the Linux start script"),
    # The heading is the platform, since "Google Play" is too wide for it.
    "google-play":      (2, "google-play", "#01875f", "#ffffff", "Android", "Google Play", "Get it on Google Play"),
    "f-droid":          (2, "f-droid", "#1976d2", "#ffffff", "Android", "F-Droid", "Get it on F-Droid"),
    "apk":              (2, "android", "#3ddc84", "#1b1b1b", "Android", "APK", "Download the Android app"),
    # One button per browser, in the colours of KnightLoader's browser tiles.
    # White holds 3.3:1 on Brave's orange, about as much as on Unraid's, and
    # more on the others. On Firefox's lighter orange it falls to 2.7:1, so
    # that ink is dark.
    "chrome":           (3, "chrome", "#1a73e8", "#ffffff", "Chrome", "Extension", "Install the extension for Chrome"),
    "edge":             (3, "edge", "#0078d7", "#ffffff", "Edge", "Extension", "Install the extension for Edge"),
    "brave":            (3, "brave", "#fb542b", "#ffffff", "Brave", "Extension", "Install the extension for Brave"),
    "opera":            (3, "opera", "#ff1b2d", "#ffffff", "Opera", "Extension", "Install the extension for Opera"),
    "vivaldi":          (3, "vivaldi", "#ef3939", "#ffffff", "Vivaldi", "Extension", "Install the extension for Vivaldi"),
    "firefox":          (3, "firefox-browser", "#ff7139", "#1b1b1b", "Firefox", "Add-on", "Install the add-on for Firefox"),
    # ParleyPort's dark is the README badges' ground. The grey is the widget
    # logo's darker half, since a one-ink mark cannot carry its colours.
    "parleyport":       (COMPANIONS, "parleyport", "#1f2328", "#ffffff", "ParleyPort", "own relay", "Get ParleyPort, the relay for KnightLoader and BombVault"),
    "bombvault-widget": (COMPANIONS, "bombvault-widget", "#4e5051", "#ffffff", "Widget", "Unraid plugin", "Get the BombVault Widget for the Unraid dashboard"),
}
# Joined to the button they belong to, in this order, rather than standing alone.
SEGMENTS = {"windows": ("windows-arm", "windows-portable"), "linux": ("linux-arm",)}
# A store listing that does not exist yet is drawn without a link.
SOON = {
    "google-play": ("Play Store soon", "On Google Play soon"),
    "f-droid": ("F-Droid soon", "On F-Droid soon"),
    "firefox": ("coming soon", "The add-on for Firefox, soon"),
    "unraid": ("coming soon", "In Unraid's Community Applications soon"),
}
# Links that may lead away from the repository.
STORES = ("play.google.com", "f-droid.org", "chromewebstore.google.com", "addons.mozilla.org", "microsoftedge.microsoft.com",
          "unraid.net", "github.com/junkerderprovinz/parleyport", "github.com/junkerderprovinz/bombvault-widget")

# The sheen is a tilted white band, clipped to each button, that appears to
# travel along the whole row, the same band as the donation row's. Its numbers
# are in screen pixels, where the gap between buttons is measured.
#
# The row is `<img width="160">` separated by a newline, two spaces and a
# `&nbsp;`, which HTML collapses to 13.16px at GitHub's 16px body text. A
# `&nbsp;` glued to `</a>` measures 8.77px, so the separator is part of the rule.
# The parts of the Windows button are glued with nothing between them.
BAND_PX = 33.0     # the band's width on screen
SPEED = 250.0      # screen pixels per second
GAP_PX = 13.16     # measured, see above
# The donation buttons' width, so every button on the page is the same size.
RENDER_PX = 160.0
# Two segments and the gap they replace make one button's place. Whole pixels,
# so no browser rounds a hairline into the seams.
SEGMENT_PX = 87.0
# A row has four places, a segment taking half of one. Five buttons already
# wrap in GitHub's 830px column and leave one alone below the rest.
PLACES = 4

SCALE = W / RENDER_PX              # canvas units per screen pixel
SEGMENT_W = SEGMENT_PX * SCALE
DIVIDER_W = 1.5 * SCALE
SHEEN_W = BAND_PX * SCALE
# skewX shifts every point by tan(16 degrees) times its y, so clearing the edge
# by the rect's width alone would leave the tilted corner showing.
SHEEN_H = H + 120.0
CLEAR = SHEEN_W + math.tan(math.radians(16)) * SHEEN_H

# The band moves linearly, since an eased pass changes speed inside each button
# and breaks the hand-off at the seam. `backwards` holds a delayed band at its
# start off the left edge; otherwise it would sit still inside the button until
# its delay ran out. The band is taller than the canvas and skewed rather than
# rotated, so the tilt never shows a corner and the motion stays one translate.
TEMPLATE = """<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}" role="img" aria-label="{alt}">
  <title>{alt}</title>
  <defs>
    <clipPath id="edge">
      <path d="{outline}"/>
    </clipPath>
    <linearGradient id="sheen" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0"    stop-color="#fff" stop-opacity="0"/>
      <stop offset="0.45" stop-color="#fff" stop-opacity="0.28"/>
      <stop offset="0.55" stop-color="#fff" stop-opacity="0.28"/>
      <stop offset="1"    stop-color="#fff" stop-opacity="0"/>
    </linearGradient>
  </defs>
  <style>
    @keyframes pass {{
      0%       {{ transform: translateX({band_start}px); }}
      {pass_pct}%   {{ transform: translateX({band_end}px); }}
      100%     {{ transform: translateX({band_end}px); }}
    }}
    .band {{ animation: pass {cycle}s linear {delay}s infinite backwards; }}
    @media (prefers-reduced-motion: reduce) {{
      .band {{ animation: none; opacity: 0; }}
    }}
  </style>
  <path d="{outline}" fill="{bg}"/>
{face}
  <g clip-path="url(#edge)">
    <g class="band">
      <rect x="0" y="-60" width="{band_w}" height="{band_h}"
            fill="url(#sheen)" transform="skewX(-16)"/>
    </g>
  </g>
</svg>
"""

FULL_FACE = """  <g transform="translate({gx} {gy}) scale({scale})" fill="{ink}">
{paths}
  </g>
  <text x="{head_x}" y="108" font-family="{font}" font-size="82" font-weight="700" fill="{ink}">{head}</text>
  <text x="{sub_x}" y="186" font-family="{font}" font-size="64" font-weight="400" fill="{ink}" fill-opacity="0.9">{sub_text}</text>"""

# A segment has no mark, and its left edge is a darker line against the part
# before it. The baselines are the full button's, so the lines read across.
SEGMENT_FACE = """  <rect width="{divider}" height="{h}" fill="#000" fill-opacity="0.28"/>
  <text x="{mid}" y="108" text-anchor="middle" font-family="{font}" font-size="72" font-weight="700" fill="{ink}">{head}</text>
  <text x="{mid}" y="186" text-anchor="middle" font-family="{font}" font-size="64" font-weight="400" fill="{ink}" fill-opacity="0.9">{sub_text}</text>"""

# Every button on the page shows one file, buttons.svg, through its own
# #svgView fragment. The shine's clock starts when an <img> gets its file, and
# separate files arrive at separate moments (Firefox also reuses a cached image
# across GitHub's in-page navigation), so the band would jump between buttons.
# The donation buttons are copied in for the same reason. The sprite layout is
# explained in junkerderprovinz/junkerderprovinz, donate/buttons/sprite.mjs.
#
# The donation buttons are read from the profile repository, so run this again
# after they change there. The sprite is read from main, so a branch's README
# preview shows main's buttons.
REPO = config.REPO
SPRITE =os.path.join(OUT, "buttons.svg")
SPRITE_URL = "https://raw.githubusercontent.com/junkerderprovinz/%s/main/.github/assets/download-buttons/buttons.svg" % REPO
GIVE_URL = "https://raw.githubusercontent.com/junkerderprovinz/junkerderprovinz/main/donate/buttons/button-%s-live.svg"
GIVE_RENDER_PX = 160.0
# slug, where it leads, accessible name. The same three as in every README.
GIVE = [
    ("buy-me-a-coffee", "https://buymeacoffee.com/junkerderprovinz", "Buy me a coffee"),
    ("paypal", "https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS", "PayPal"),
    ("crypto", "https://junkerderprovinz.github.io/junkerderprovinz/", "Donate with crypto"),
]
README = os.path.join(HERE, "..", "README.md")
ROW_OPEN = "<!-- download-buttons: written by scripts/gen_download_buttons.py -->"
ROW_CLOSE = "<!-- /download-buttons -->"
# Inside the paragraph of the last row that downloads a release file, so it
# sits right under those buttons rather than a paragraph margin away.
CAPTION = "Always downloads the latest build"
LATEST = "/releases/latest/download/"
GIVE_OPEN = "<!-- give-buttons: written by scripts/gen_download_buttons.py -->"
GIVE_CLOSE = "<!-- /give-buttons -->"
# The Android row again, in the README's section on the app.
APP_OPEN = "<!-- app-buttons: written by scripts/gen_download_buttons.py -->"
APP_CLOSE = "<!-- /app-buttons -->"
APP_ROW = 2


class Part:
    """One image in a row: a whole button, or one piece of a segmented one."""

    def __init__(self, kind, href, width, corners):
        self.kind, self.href, self.width, self.corners = kind, href, width, corners
        _row, self.mark, self.bg, self.ink, self.head, self.sub, self.alt = KINDS[kind]
        if href is None:
            self.sub, self.alt = SOON[kind]
        self.px = width / SCALE


def rows():
    """The configured buttons as rows of items, an item being the parts drawn
    as one button: one part, or a button and its segments."""
    unknown = sorted(set(config.BUTTONS) - set(KINDS))
    if unknown:
        raise SystemExit("download_buttons.py names buttons this file cannot draw: %s" % ", ".join(unknown))
    for kind, href in config.BUTTONS.items():
        if href is None and kind not in SOON:
            raise SystemExit("%s has no link" % kind)
        if href and "/%s/" % REPO not in href and not any(host in href for host in STORES):
            raise SystemExit("REPO is %r, but %s leads to %s" % (REPO, kind, href))
    joined = {s for group in SEGMENTS.values() for s in group}
    for base, group in SEGMENTS.items():
        present = [s for s in group if s in config.BUTTONS]
        if present and base not in config.BUTTONS:
            raise SystemExit("%s needs the %s button to join" % (present[0], base))
    out = [[], [], [], [], []]
    for kind, spec in KINDS.items():
        if kind not in config.BUTTONS or kind in joined:
            continue
        href = config.BUTTONS[kind]
        segments = [s for s in SEGMENTS.get(kind, ()) if s in config.BUTTONS]
        if segments:
            item = [Part(kind, href, W, "left")]
            item += [Part(s, config.BUTTONS[s], SEGMENT_W, "right" if s == segments[-1] else "none") for s in segments]
        else:
            item = [Part(kind, href, W, "all")]
        out[spec[0]].append(item)
    return [row for items in out if items for row in split(items)]


def split(items):
    """A group as rows of at most PLACES, each filled before the next begins."""
    out, used = [[]], 0.0
    for parts in items:
        size = 1 + (len(parts) - 1) / 2
        if out[-1] and used + size > PLACES:
            out.append([])
            used = 0.0
        out[-1].append(parts)
        used += size
    return out


def outline(w, corners):
    """The button's shape: rounded on both sides, or square where it meets a segment."""
    left = R if corners in ("all", "left") else 0
    right = R if corners in ("all", "right") else 0
    d = ["M%s,0" % num(left), "H%s" % num(w - right)]
    if right:
        d.append("A%s,%s 0 0 1 %s,%s" % (R, R, num(w), R))
    d.append("V%s" % num(H - right))
    if right:
        d.append("A%s,%s 0 0 1 %s,%s" % (R, R, num(w - right), num(H)))
    d.append("H%s" % num(left))
    if left:
        d.append("A%s,%s 0 0 1 0,%s" % (R, R, num(H - left)))
    d.append("V%s" % num(left))
    if left:
        d.append("A%s,%s 0 0 1 %s,0" % (R, R, num(left)))
    return " ".join(d) + " Z"


def brand(name, ink):
    """One mark as markup in the button's ink, and the scale and offsets that
    centre it in GLYPH."""
    paths = [p for p in io.open(os.path.join(BRANDS, name + ".txt"), encoding="utf-8").read().splitlines() if p.strip()]
    box = io.open(os.path.join(BRANDS, name + ".box.txt"), encoding="utf-8").read().strip()
    x, y, width, height = (float(n) for n in box.split())
    # Scaled by height so the marks share an optical size, then nudged right by
    # half the width they do not use, since Apple's mark is narrower. A mark
    # wider than WIDEST is scaled by width and lowered by half the height it
    # leaves. The box's origin is taken off, since not every viewBox starts at 0.
    scale = min(GLYPH / height, WIDEST / width) * LARGER.get(name, 1.0)
    seam = ' stroke="%s" stroke-width="%s"' % (ink, num(SEAM / scale)) if len(paths) > 1 else ""
    markup = "\n".join('    <path d="%s"%s/>' % (p, seam) for p in paths)
    return markup, scale, (GLYPH - width * scale) / 2 - x * scale, (GLYPH - height * scale) / 2 - y * scale


def num(x):
    """A coordinate as the fragment carries it: no trailing zeros, no float noise."""
    return ("%.3f" % x).rstrip("0").rstrip(".")


def button(part, delay, cycle):
    """One part's own SVG document, its band starting after `delay`."""
    common = dict(font=FONT, ink=part.ink, head=part.head, sub_text=part.sub)
    if part.mark:
        markup, scale, dx, dy = brand(part.mark, part.ink)
        face = FULL_FACE.format(gx=round(GX + dx, 2), gy=round(GY + dy, 2), scale=round(scale, 5), paths=markup,
                                head_x=TEXT_X, sub_x=TEXT_X + 2, **common)
    else:
        face = SEGMENT_FACE.format(divider=num(DIVIDER_W), h=H, mid=num(part.width / 2), **common)
    crossing = (part.width + 2 * CLEAR) / SCALE / SPEED
    return TEMPLATE.format(
        w=num(part.width), h=H, bg=part.bg, alt=escape(part.alt), outline=outline(part.width, part.corners),
        face=face, delay="%.3f" % delay, cycle="%g" % cycle, pass_pct="%.2f" % (crossing / cycle * 100.0),
        band_w="%.1f" % SHEEN_W, band_h="%g" % SHEEN_H, band_start="%.1f" % -CLEAR,
        band_end="%.1f" % (part.width + CLEAR),
    )


SPRITE_GAP = 24

# The names a button document defines, each prefixed per button in the sprite. A
# name left without a prefix is refused, since it would hand one button's delay
# or clip to all of them.
UNPREFIXED = re.compile(r'id="(?!b\d+-)|url\(#(?!b\d+-)|href="#(?!b\d+-)|class="(?!b\d+-)|@keyframes (?!b\d+-)|animation: (?!b\d+-|none)')


def sprite(parts):
    """Several button documents as one SVG, laid out left to right.

    Every part keeps its own document as a nested <svg> at its own x, with its
    ids, class and keyframes prefixed, because the CSS inside one SVG document
    is shared: unprefixed, the last button's delay would win for all of them.
    Parts start on whole units with SPRITE_GAP of nothing between them: a
    browser scaling a cut-out by a fractional factor samples a pixel past its
    edge, and with the neighbour right there that pixel shows as a coloured
    line along the button.
    Returns the sprite and each part's x.
    """
    x, xs, body = 0.0, [], []
    for i, (svg, width, _height) in enumerate(parts):
        pre = "b%d-" % i
        s = re.sub(r"<\?xml[^>]*>\s*", "", svg, count=1)
        s = re.sub(r"<!--.*?-->\s*", "", s, flags=re.S)
        s = re.sub(r'id="(edge|sheen)"', lambda m: 'id="%s%s"' % (pre, m.group(1)), s)
        s = re.sub(r"url\(#(edge|sheen)\)", lambda m: "url(#%s%s)" % (pre, m.group(1)), s)
        s = s.replace('class="band"', 'class="%sband"' % pre)
        s = re.sub(r"\.band\b", ".%sband" % pre, s)
        s = re.sub(r"@keyframes pass\b", "@keyframes %spass" % pre, s)
        s = s.replace("animation: pass ", "animation: %spass " % pre)
        s = re.sub(r"<svg\b", '<svg x="%s" y="0"' % num(x), s, count=1)
        left = UNPREFIXED.search(s)
        if left:
            raise SystemExit("button %d still has an unprefixed name near %r" % (i, s[left.start():left.start() + 40]))
        xs.append(x)
        body.append(s.strip())
        x = float(math.ceil(x + width + SPRITE_GAP))
    height = max(p[2] for p in parts)
    head = ('<?xml version="1.0" encoding="UTF-8"?>\n'
            '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" '
            'viewBox="0 0 %s %s">\n' % (num(x), num(height)))
    return head + "\n".join(body) + "\n</svg>\n", xs


def give_buttons():
    """The donation buttons' own files and sizes, as the profile repository publishes them.

    The size is read from each file's viewBox rather than assumed, because the
    other repository decides it. The query string gets past raw.githubusercontent's
    five-minute cache, so a run right after a change there sees the change.
    """
    out = []
    for slug, _href, _alt in GIVE:
        url = GIVE_URL % slug + "?t=%d" % time.time()
        for attempt in range(3):
            try:
                with urllib.request.urlopen(url, timeout=30) as r:
                    svg = r.read().decode("utf-8")
                break
            except urllib.error.HTTPError as err:
                if err.code < 500 or attempt == 2:
                    raise SystemExit("%s: HTTP %d" % (url, err.code))
            except (OSError, http.client.HTTPException):
                # A dropped connection, also halfway through the body. Nothing
                # has been written yet, so trying again is safe.
                if attempt == 2:
                    raise
            time.sleep(2)
        box = re.search(r'viewBox="0 0 ([\d.]+) ([\d.]+)"', svg)
        if not box:
            raise SystemExit("%s has no viewBox" % url)
        out.append((svg, float(box.group(1)), float(box.group(2))))
    return out


def blocks(text, opener, closer):
    """Every opener ... closer span in the text, in order.

    An opener whose closer is missing is refused rather than paired with the
    next block's closer, which would replace everything in between.
    """
    spans, at = [], 0
    while True:
        start = text.find(opener, at)
        if start < 0:
            return spans
        end = text.find(closer, start)
        following = text.find(opener, start + len(opener))
        if end < 0 or (0 <= following < end):
            raise SystemExit("README.md has %s without its %s" % (opener, closer))
        spans.append((start, end))
        at = end + len(closer)


def read_readme():
    """README.md, read after the donation buttons are fetched so an edit saved
    meanwhile survives, and checked before any file is written."""
    text = io.open(README, encoding="utf-8", newline="").read()
    if len(blocks(text, ROW_OPEN, ROW_CLOSE)) != 1:
        raise SystemExit("README.md needs exactly one %s ... %s" % (ROW_OPEN, ROW_CLOSE))
    if not blocks(text, GIVE_OPEN, GIVE_CLOSE):
        raise SystemExit("README.md has no %s ... %s" % (GIVE_OPEN, GIVE_CLOSE))
    return text


def row(items, nl, url, caption=None):
    """One centred row: a link per image, the separator on its own line, two
    spaces in, because that is the gap GAP_PX was measured on. The parts of one
    button share a line with nothing between them."""
    lines = ['<p align="center">']
    for index, parts in enumerate(items):
        if index:
            lines.append("  &nbsp;")
        imgs = []
        for href, alt, x, width, height, render in parts:
            img = ('<img src="%s#svgView(viewBox(%s,0,%s,%s))" alt="%s" width="%s" height="%s">'
                   % (url, num(x), num(width), num(height), escape(alt), num(render), num(render * height / width)))
            imgs.append('<a href="%s">%s</a>' % (escape(href), img) if href else img)
        lines.append("  " + "".join(imgs))
    if caption:
        lines.append("  <br><sub>%s</sub>" % caption)
    lines.append("</p>")
    return nl.join(lines) + nl


def write_readme(text, downloads, groups, apps, donations, url):
    """Replace every marked block, each taking the line ending of its own marker.

    Both width and height are set, because the image's own proportions are the
    whole sprite's, not the button's.
    """
    blocks_out = ((ROW_OPEN, ROW_CLOSE, downloads, groups, CAPTION), (APP_OPEN, APP_CLOSE, apps, [APP_ROW] * len(apps), CAPTION),
                  (GIVE_OPEN, GIVE_CLOSE, donations, [0], None))
    for opener, closer, table, group, caption in blocks_out:
        for start, end in reversed(blocks(text, opener, closer)):
            nl = "\r\n" if text[start:].split("\n", 1)[0].endswith("\r") else "\n"
            last = max((i for i, items in enumerate(table) if any(LATEST in p[0] for parts in items for p in parts if p[0])), default=None)
            # A <br> between paragraphs is the one spacer GitHub keeps, and it
            # sets a new group about twice as far off as the next row.
            body = "".join(("<br>" + nl if i and group[i] != group[i - 1] else "") + row(items, nl, url, caption if i == last else None)
                           for i, items in enumerate(table))
            text = text[:start] + opener + nl + body + text[end:]
    io.open(README, "w", encoding="utf-8", newline="").write(text)
    print("README.md  download rows of %s, donation rows of %d"
          % ("+".join(str(sum(len(item) for item in r)) for r in downloads), len(GIVE)))


ANIMATION = re.compile(r"animation: pass (\d+(?:\.\d+)?)s linear (\d+(?:\.\d+)?)s infinite backwards;")
PASS_STOP = re.compile(r"^(\s*)(\d+(?:\.\d+)?)%(\s+\{ transform: translateX\()", re.M)


def retime(svg, delay, cycle):
    """A donation button moved to its place in this page's loop.

    The band's time across the button is read from the file, since its geometry
    belongs to the profile repository. Anything but the one expected animation
    and keyframe stops the run rather than leaving one button on its old clock.
    """
    found = ANIMATION.findall(svg)
    stops = [m for m in PASS_STOP.finditer(svg) if float(m.group(2)) not in (0.0, 100.0)]
    if len(found) != 1 or len(stops) != 1:
        raise SystemExit("a donation button lacks the one animation and keyframe this retimes")
    crossing = float(stops[0].group(2)) / 100.0 * float(found[0][0])
    stop = stops[0]
    svg = svg[:stop.start()] + "%s%.2f%%%s" % (stop.group(1), crossing / cycle * 100.0, stop.group(3)) + svg[stop.end():]
    return ANIMATION.sub("animation: pass %gs linear %.3fs infinite backwards;" % (cycle, delay), svg)


def layout(items, gap):
    """Each part's x within its row in screen pixels, and how far the band
    travels before the next row takes over: the row and one more gap."""
    x, placed = 0.0, []
    for parts in items:
        for part in parts:
            placed.append((part, x))
            x += part.px
        x += gap
    return placed, x


# One band works down the page, row by row in the order the README shows them:
# the download rows and the donation row, whichever stands higher first. Within
# a row each part starts when the band reaches its left edge, and each row where
# a further button of the row above would have started. Computed, so moving a
# row or a button cannot break the hand-off.
#
# The house loop is seven seconds, with the donation row at 3.8 s and a rest of
# 1.12 s after it. A page whose rows need longer keeps that rest and stretches
# the loop. Every delay stays inside the loop: all clocks start together, and a
# delay past the loop would make a button shine before everything above it.
HOUSE_CYCLE, HOUSE_GIVE_START = 7.0, 3.8
GIVE_STEP = (GIVE_RENDER_PX + GAP_PX) / SPEED
REST = HOUSE_CYCLE - (HOUSE_GIVE_START + len(GIVE) * GIVE_STEP)


def main():
    table = rows()
    gives = give_buttons()
    readme = read_readme()

    downloads = [layout(items, GAP_PX) for items in table]
    donation = layout([[SimpleNamespace(px=GIVE_RENDER_PX)] for _ in GIVE], GAP_PX)
    order = downloads + [donation]
    if readme.find(GIVE_OPEN) < readme.find(ROW_OPEN):
        order = [donation] + downloads
    starts, at = {}, 0.0
    for placed, travel in order:
        for part, x in placed:
            starts[id(part)] = at + x / SPEED
        at += travel / SPEED
    cycle = round(max(HOUSE_CYCLE, at + REST), 3)

    parts = [part for placed, _ in downloads for part, _x in placed]
    svgs = [button(part, starts[id(part)], cycle) for part in parts]
    gives = [(retime(svg, starts[id(part)], cycle), width, height)
             for (part, _x), (svg, width, height) in zip(donation[0], gives)]
    # Built, and checked, before anything is written.
    whole, xs = sprite([(svg, part.width, H) for part, svg in zip(parts, svgs)] + gives)

    os.makedirs(OUT, exist_ok=True)
    written = {os.path.normcase(os.path.abspath(SPRITE))}
    for part, svg in zip(parts, svgs):
        out = os.path.join(OUT, "button-" + part.kind + ".svg")
        io.open(out, "w", encoding="utf-8", newline="\n").write(svg)
        written.add(os.path.normcase(os.path.abspath(out)))
        print("wrote", os.path.normpath(out), len(svg), "bytes")
    io.open(SPRITE, "w", encoding="utf-8", newline="\n").write(whole)
    print("wrote", os.path.normpath(SPRITE), len(whole), "bytes,", len(xs), "buttons")
    # The folder is this file's alone, so a button taken off the list leaves no file behind.
    for old in glob.glob(os.path.join(OUT, "*.svg")):
        if os.path.normcase(os.path.abspath(old)) not in written:
            os.remove(old)
            print("removed", os.path.normpath(old))

    place = iter(xs)
    table_out = [[[(p.href, p.alt, next(place), p.width, H, p.px) for p in item] for item in items] for items in table]
    apps = [out for items, out in zip(table, table_out) if KINDS[items[0][0].kind][0] == APP_ROW]
    donations = [[[(href, alt, xs[len(parts) + i], gives[i][1], gives[i][2], GIVE_RENDER_PX)]
                  for i, (_s, href, alt) in enumerate(GIVE)]]
    # Browsers and GitHub keep the sprite for a while, and an old sprite cut at
    # new offsets shows the wrong buttons, so its address changes with it.
    url = "%s?v=%s" % (SPRITE_URL, hashlib.sha256(whole.encode("utf-8")).hexdigest()[:12])
    groups = [KINDS[items[0][0].kind][0] for items in table]
    write_readme(readme, table_out, groups, apps, donations, url)


if __name__ == "__main__":
    main()
