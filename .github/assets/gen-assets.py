"""
Step 2: composite the logo onto a white 1600×500 banner.
Requires Pillow. Run after gen-assets.mjs.
"""
from pathlib import Path
from PIL import Image

ASSETS = Path(__file__).parent

logo = Image.open(ASSETS / "_banner-logo-tmp.png").convert("RGBA")

BANNER_W, BANNER_H = 1600, 500
banner = Image.new("RGBA", (BANNER_W, BANNER_H), (255, 255, 255, 255))

x = (BANNER_W - logo.width) // 2
y = (BANNER_H - logo.height) // 2
banner.paste(logo, (x, y), logo)

banner.convert("RGB").save(ASSETS / "bombvault-banner.png", "PNG", optimize=True)
print(f"bombvault-banner.png written ({BANNER_W}×{BANNER_H})")

(ASSETS / "_banner-logo-tmp.png").unlink(missing_ok=True)
print("temp file removed")
