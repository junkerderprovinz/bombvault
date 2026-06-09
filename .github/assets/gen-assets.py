"""
Compose step for the BombVault assets (run after gen-assets.mjs).

  icon.png             : outer background made transparent, interior white
                         outlines preserved (so it looks right on Unraid's dark
                         Docker tab AND on white CA cards). 512px wide.
  bombvault-banner.png : logo centered on a white 1600x500 banner (white per the
                         GitHub style guide).

Requires Pillow.
"""
from pathlib import Path
from collections import deque
from PIL import Image

A = Path(__file__).parent


def transparent_outer(src: Image.Image) -> Image.Image:
    """Flood-fill the white background from the edges to transparent. Interior
    white (enclosed by the dark frame/bomb) is unreachable and stays opaque."""
    img = src.convert("RGBA")
    w, h = img.size
    px = img.load()

    def is_bg(x, y):
        r, g, b, _ = px[x, y]
        # near-white + its AA halo, but NOT the #c6c6c6 (198) panel
        return r >= 235 and g >= 235 and b >= 235

    visited = bytearray(w * h)
    dq = deque()
    for x in range(w):
        for y in (0, h - 1):
            if is_bg(x, y) and not visited[y * w + x]:
                visited[y * w + x] = 1
                dq.append((x, y))
    for y in range(h):
        for x in (0, w - 1):
            if is_bg(x, y) and not visited[y * w + x]:
                visited[y * w + x] = 1
                dq.append((x, y))
    while dq:
        x, y = dq.popleft()
        r, g, b, _ = px[x, y]
        px[x, y] = (r, g, b, 0)
        for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
            nx, ny = x + dx, y + dy
            if 0 <= nx < w and 0 <= ny < h and not visited[ny * w + nx] and is_bg(nx, ny):
                visited[ny * w + nx] = 1
                dq.append((nx, ny))
    return img


# icon.png — transparent outer, downscaled to 512 wide
hi = Image.open(A / "icon_white_hi.png")
icon = transparent_outer(hi)
icon = icon.resize((512, round(512 * hi.height / hi.width)), Image.LANCZOS)
icon.save(A / "icon.png")
print(f"icon.png written {icon.size} (transparent outer)")

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
