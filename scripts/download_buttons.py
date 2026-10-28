"""The download buttons the README shows, read by gen_download_buttons.py.

Each entry names a button the generator knows and where it leads. The rows,
their order, the colours and the words are the generator's, the same in every
repository.
"""

REPO = "bombvault"

BUTTONS = {
    "unraid": "https://ca.unraid.net/apps/bombvault-06cz2zq1itlcy2",
    "mos": "https://mos-official.net/apps.html?search=BombVault",
    "zimaos": "https://github.com/junkerderprovinz/zimaos-apps#bombvault",
    # A browser cannot download an image, so this opens the package page, which
    # carries the pull command and every tag.
    "compose": "https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/deploy/docker-compose.generic.yml",
    "docs": "https://junkerderprovinz.github.io/bombvault/",
    # Google Play has no public listing yet, so its button is drawn without a
    # link. The README's call for testers leads to the closed test meanwhile.
    "google-play": None,
    "f-droid": "https://f-droid.org/packages/bombvault.halleluja.design/",
    "apk": "https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk",
    "parleyport": "https://github.com/junkerderprovinz/parleyport",
    "bombvault-widget": "https://github.com/junkerderprovinz/bombvault-widget",
}
