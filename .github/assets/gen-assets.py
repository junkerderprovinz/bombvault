"""
Compose step for the BombVault container/CA icon (run after gen-assets.mjs).

  icon.png : the hand-made transparent icon (icon-source.png), premultiplied-alpha
             downscaled to 512px wide so the transparent edges stay clean (NO white
             halo) on Unraid's dark Docker tab. The user wants the container logo
             transparent — see the ca-icon-background note.

The banner (bombvault-banner.png) is rendered from the self-contained
bombvault-banner.svg by gen-assets.mjs; it is no longer composed here.

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
