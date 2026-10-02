# Marks on the download buttons

The path data `../gen_download_buttons.py` draws into the README's download
buttons. `<name>.txt` is the `d` attribute of the mark's single path,
`<name>.box.txt` the viewBox it was drawn in, which is what lets the generator
scale marks of different widths to one optical size.

## Source and licence

**Font Awesome Free 6.7.2**, from <https://fontawesome.com>: Docker from the
`brands` set; the ZIP (`file-zipper`), the book (`book`), the relay
(`tower-broadcast`) and the widget (`table-cells-large`) from the `solid` set.
The icons are **CC BY 4.0**, which asks for attribution and nothing else.
Copyright 2024 Fonticons, Inc.

**Dashboard Icons** (`homarr-labs/dashboard-icons`, <https://dashboardicons.com>):
Unraid, its `unraid.svg` with the gradient left out, since the button draws it
in one ink. **Apache-2.0**, whose licence text is beside this file as
`LICENSE-dashboard-icons.txt`. Copyright the Homarr Labs team and contributors.

## Trademarks

Every platform mark here is a trademark of its owner. They are used the one way a
trademark may be used without permission, which is to refer to the thing they
name: each sits on a download button for that platform, unmodified, and nothing
here claims endorsement by or affiliation with Docker or Lime Technology. The
ZIP, the book, the mast and the tiles are no one's marks; they stand for the
source archive, the manual, the relay and the dashboard widget.

## Adding one

Take the SVG, keep its `viewBox` verbatim in `<name>.box.txt`, and put the `d`
attribute of its single path in `<name>.txt`. A mark needing more than one path
needs a change to the generator's template as well, because these buttons draw
their marks in one ink.
