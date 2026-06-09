"""
Compose step for the BombVault assets (run after gen-assets.mjs).

  icon.png             : the icon on a SOLID WHITE background, 512px wide. A
                         transparent background made the dark bomb + dark safe
                         frame blend into Unraid's dark CA / Docker UI, so the
                         icon is kept on a solid white card instead.
  bombvault-banner.png : logo centered on a white 1600x500 banner (white per the
                         GitHub style guide).

Requires Pillow.
"""
from pathlib import Path
from PIL import Image

A = Path(__file__).parent


# icon.png — solid white background, downscaled to 512 wide. Dark subject on a
# dark UI needs a solid light backdrop, never transparent (see ca-icon-background).
hi = Image.open(A / "icon_white_hi.png").convert("RGB")
icon = hi.resize((512, round(512 * hi.height / hi.width)), Image.LANCZOS)
icon.save(A / "icon.png")
print(f"icon.png written {icon.size} (solid white)")

# bombvault-banner.png — logo centered on white 1600x500
logo = Image.open(A / "_banner-logo-tmp.png").convert("RGBA")
BW, BH = 1600, 500
banner = Image.new("RGBA", (BW, BH), (255, 255, 255, 255))
banner.paste(logo, ((BW - logo.width) // 2, (BH - logo.height) // 2), logo)
banner.convert("RGB").save(A / "bombvault-banner.png", "PNG", optimize=True)
print(f"bombvault-banner.png written ({BW}x{BH})")

# clean temps
(A / "icon_white_hi.png").unlink(missing_ok=True)
(A / "_banner-logo-tmp.png").unlink(missing_ok=True)
print("temps removed")
