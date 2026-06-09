"""
Compose step for the BombVault assets (run after gen-assets.mjs).

  icon.png             : the hand-made transparent icon (icon-source.png),
                         premultiplied-alpha downscaled to 512px wide so the
                         transparent edges stay clean (NO white halo) on Unraid's
                         dark Docker tab. The user wants the container logo
                         transparent — see the ca-icon-background note.
  bombvault-banner.png : logo centered on a white 1600x500 banner (white per the
                         GitHub style guide).

Requires Pillow.
"""
from pathlib import Path
from PIL import Image

A = Path(__file__).parent


def premultiplied_resize(src: Image.Image, width: int) -> Image.Image:
    """Downscale RGBA with premultiplied alpha, so a transparent (white-backed)
    source does not bleed a light halo into the anti-aliased edges."""
    src = src.convert("RGBA")
    r, g, b, a = src.split()
    black = Image.new("RGB", src.size, (0, 0, 0))
    pr, pg, pb = Image.composite(Image.merge("RGB", (r, g, b)), black, a).split()
    prem = Image.merge("RGBA", (pr, pg, pb, a))
    h = round(width * src.height / src.width)
    out = prem.resize((width, h), Image.LANCZOS).convert("RGBA")
    px = out.load()
    for y in range(h):
        for x in range(width):
            rr, gg, bb, aa = px[x, y]
            if aa == 0:
                px[x, y] = (0, 0, 0, 0)
            elif aa < 255:  # un-premultiply
                px[x, y] = (min(255, (rr * 255 + aa // 2) // aa),
                            min(255, (gg * 255 + aa // 2) // aa),
                            min(255, (bb * 255 + aa // 2) // aa), aa)
    return out


# icon.png — the user's transparent source, cleanly downscaled to 512 wide.
icon = premultiplied_resize(Image.open(A / "icon-source.png"), 512)
icon.save(A / "icon.png")
print(f"icon.png written {icon.size} (transparent, from icon-source.png)")

# bombvault-banner.png — logo centered on white 1600x500
logo = Image.open(A / "_banner-logo-tmp.png").convert("RGBA")
BW, BH = 1600, 500
banner = Image.new("RGBA", (BW, BH), (255, 255, 255, 255))
banner.paste(logo, ((BW - logo.width) // 2, (BH - logo.height) // 2), logo)
banner.convert("RGB").save(A / "bombvault-banner.png", "PNG", optimize=True)
print(f"bombvault-banner.png written ({BW}x{BH})")

# clean temps
(A / "_banner-logo-tmp.png").unlink(missing_ok=True)
print("temps removed")
