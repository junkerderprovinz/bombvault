"""The download buttons the README shows, read by gen_download_buttons.py.

Each entry names a button the generator knows and where it leads. The rows,
their order, the colours and the words are the generator's, the same in every
repository.
"""

REPO = "bombvault"

BUTTONS = {
    # A browser cannot download an image, so this opens the package page, which
    # carries the pull command and every tag.
    "docker": "https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault",
    # A release's "Source code (zip)" is the whole repository at that tag, and
    # GitHub gives the newest one no fixed address, so this leads to the release
    # that lists it.
    "source": "https://github.com/junkerderprovinz/bombvault/releases/latest",
    "docs": "https://junkerderprovinz.github.io/bombvault/",
}
