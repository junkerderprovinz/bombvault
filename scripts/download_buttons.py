"""The download buttons the README shows, read by gen_download_buttons.py.

Each entry names a button the generator knows and where it leads. The rows,
their order, the colours and the words are the generator's, the same in every
repository.
"""

REPO = "bombvault"

BUTTONS = {
    "unraid": "https://unraid.net/community/apps?q=bombvault",
    # A browser cannot download an image, so this opens the package page, which
    # carries the pull command and every tag.
    "docker": "https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault",
    "docs": "https://junkerderprovinz.github.io/bombvault/",
    "relay": "https://github.com/junkerderprovinz/parleyport",
    "widget": "https://github.com/junkerderprovinz/bombvault-widget",
}
