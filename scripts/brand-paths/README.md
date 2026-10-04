# Marks on the download buttons

The path data `../gen_download_buttons.py` draws into the README's download
buttons. `<name>.txt` holds the `d` attribute of each of the mark's paths, one
per line, and `<name>.box.txt` the viewBox it was drawn in, which is what lets
the generator scale marks of different widths to one optical size.

## Source and licence

**Font Awesome Free 6.7.2**, from <https://fontawesome.com>: Docker, Android
and Google Play from the `brands` set; the ZIP (`file-zipper`) and the book
(`book`) from the `solid` set. The icons are **CC BY 4.0**, which asks for
attribution and nothing else. Copyright 2024 Fonticons, Inc.

**Simple Icons 16.34.0** (<https://simpleicons.org>): F-Droid, in one ink, since
the Dashboard Icons drawing is shaded in several tones. **CC0 1.0**.

**Dashboard Icons** (`homarr-labs/dashboard-icons`, <https://dashboardicons.com>):
Unraid, its `unraid.svg` with the gradient left out, since the button draws it
in one ink. **Apache-2.0**, whose licence text is beside this file as
`LICENSE-dashboard-icons.txt`. Copyright the Homarr Labs team and contributors.

The widget mark is the BombVault Widget's own logo, from
<https://github.com/junkerderprovinz/bombvault-widget>, one path per shape.

ParleyPort's mark is its own logo, from
<https://github.com/junkerderprovinz/parleyport>, one path per shape.

## Trademarks

Every platform mark here is a trademark of its owner. They are used the one way a
trademark may be used without permission, which is to refer to the thing they
name: each sits on a download button for that platform, unmodified, and nothing
here claims endorsement by or affiliation with Docker, Google, F-Droid or Lime
Technology. The ZIP and the book are no one's marks; they stand for the source
archive and the manual.

## Adding one

Take the SVG, keep its `viewBox` verbatim in `<name>.box.txt`, and put the `d`
attribute of each path on a line of its own in `<name>.txt`, a `rect` written as
a path. The button draws every path in one ink, so the mark's colours are lost.
