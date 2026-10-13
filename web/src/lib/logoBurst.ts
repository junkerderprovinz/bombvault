// The logo flies apart as the picture it is: its own tiles as shards, with a
// fireball, sparks, a shock ring and smoke drawn on a canvas over the page.

const TAU = Math.PI * 2;
const GRID = 14;
const SPARKS = 120;
const PUFFS = 18;
const GRAVITY = 1500;

/** BURST_SECONDS is how long the canvas stays on the page. */
export const BURST_SECONDS = 1.9;

type Stop = [number, string];

const rnd = (lo: number, hi: number) => lo + Math.random() * (hi - lo);
const ease = (k: number) => 1 - (1 - k) ** 3;

/** The level's reach: how far the debris flies and how large the fire grows. */
function reach(): number {
  const v = parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--motion-burst-reach"));
  return Number.isFinite(v) ? v : 1;
}

/**
 * The drawn mark rasterised once at canvas resolution, so the shards slice
 * pixels rather than redrawing the SVG 196 times a frame. An SVG without width
 * and height has no natural size in some browsers, and then there is nothing
 * to slice.
 */
function raster(img: HTMLImageElement, w: number, h: number, dpr: number): HTMLCanvasElement | null {
  if (!img.complete || img.naturalWidth === 0 || img.naturalHeight === 0) return null;
  const out = document.createElement("canvas");
  out.width = Math.max(1, Math.round(w * dpr));
  out.height = Math.max(1, Math.round(h * dpr));
  const g = out.getContext("2d");
  if (!g) return null;
  try {
    g.drawImage(img, 0, 0, out.width, out.height);
  } catch {
    return null;
  }
  return out;
}

/**
 * logoBurst blows up the image on a full-window canvas and calls done once
 * the debris has settled and the canvas is gone. The returned function stops
 * it early and removes the canvas without calling done.
 */
export function logoBurst(img: HTMLImageElement, done: () => void): () => void {
  const box = img.getBoundingClientRect();
  const cx = box.left + box.width / 2;
  const cy = box.top + box.height / 2;
  const size = Math.max(box.width, box.height);
  const far = reach();

  // object-contain letterboxes a mark that is not square, so the shards come
  // from the drawn rectangle rather than from the element's box.
  const aspect = img.naturalWidth > 0 && img.naturalHeight > 0 ? img.naturalWidth / img.naturalHeight : 1;
  const drawnW = Math.min(box.width, box.height * aspect);
  const drawnH = drawnW / aspect;
  const left = cx - drawnW / 2;
  const top = cy - drawnH / 2;

  const cv = document.createElement("canvas");
  const dpr = Math.min(2, window.devicePixelRatio || 1);
  cv.className = "glim-logo-burst";
  cv.setAttribute("aria-hidden", "true");
  cv.width = innerWidth * dpr;
  cv.height = innerHeight * dpr;
  document.body.appendChild(cv);
  // Drawn on the CPU, since a window-sized canvas on the GPU breaks the
  // drawing of Android's WebView.
  const g = cv.getContext("2d", { willReadFrequently: true });
  const src = raster(img, drawnW, drawnH, dpr);

  const cellW = drawnW / GRID;
  const cellH = drawnH / GRID;
  const shards: {
    sx: number; sy: number; x: number; y: number; vx: number; vy: number; spin: number; grow: number;
  }[] = [];
  if (src) {
    for (let y = 0; y < GRID; y++) {
      for (let x = 0; x < GRID; x++) {
        const dx = (x + 0.5) / GRID - 0.5;
        const dy = (y + 0.5) / GRID - 0.5;
        const dir = Math.atan2(dy, dx) + rnd(-0.45, 0.45);
        const v = rnd(320, 1050) * (0.55 + Math.hypot(dx, dy)) * far;
        shards.push({
          sx: (x * src.width) / GRID,
          sy: (y * src.height) / GRID,
          x: left + x * cellW + cellW / 2,
          y: top + y * cellH + cellH / 2,
          vx: Math.cos(dir) * v,
          vy: Math.sin(dir) * v - rnd(80, 320) * far,
          spin: rnd(-16, 16),
          grow: rnd(0.8, 1.5),
        });
      }
    }
  }
  const sparks = Array.from({ length: SPARKS }, () => {
    const dir = rnd(0, TAU);
    const v = rnd(250, 1300) * far;
    return { vx: Math.cos(dir) * v, vy: Math.sin(dir) * v, life: rnd(0.3, 0.95), w: rnd(1.2, 2.8) };
  });
  const smoke = Array.from({ length: PUFFS }, () => {
    const dir = rnd(0, TAU);
    const d = rnd(0, size * 0.6);
    return {
      x: cx + Math.cos(dir) * d,
      y: cy + Math.sin(dir) * d,
      vx: Math.cos(dir) * rnd(20, 110) * far,
      vy: -rnd(30, 120) * far,
      r: rnd(size * 0.35, size * 0.9) * far,
      life: rnd(1, 1.7),
    };
  });

  const ball = (x: number, y: number, r: number, stops: Stop[]) => {
    if (!g || r <= 0) return;
    const grad = g.createRadialGradient(x, y, 0, x, y, r);
    for (const [at, c] of stops) grad.addColorStop(at, c);
    g.fillStyle = grad;
    g.beginPath();
    g.arc(x, y, r, 0, TAU);
    g.fill();
  };

  const draw = (t: number) => {
    if (!g) return;
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    g.clearRect(0, 0, innerWidth, innerHeight);
    for (const p of smoke) {
      const k = t / p.life;
      if (k >= 1 || t < 0.12) continue;
      g.globalAlpha = 0.42 * (1 - k) * Math.min(1, (t - 0.12) * 6);
      ball(p.x + p.vx * t, p.y + p.vy * t, p.r * (0.5 + k), [
        [0, "rgba(40,36,34,1)"],
        [0.6, "rgba(40,36,34,.55)"],
        [1, "rgba(40,36,34,0)"],
      ]);
    }
    g.globalAlpha = 1;
    g.globalCompositeOperation = "lighter";
    if (t < 0.75) {
      const k = t / 0.75;
      const r = size * (0.5 + 3.4 * ease(k)) * far;
      const fade = (1 - k) ** 1.6;
      ball(cx, cy, r, [
        [0, `rgba(255,255,235,${fade})`],
        [0.22, `rgba(255,226,120,${fade})`],
        [0.5, `rgba(255,140,30,${fade * 0.9})`],
        [0.78, `rgba(210,50,10,${fade * 0.55})`],
        [1, "rgba(120,20,0,0)"],
      ]);
      for (let i = 0; i < 6; i++) {
        const dir = i * 1.05 + 1;
        const d = r * 0.42;
        ball(cx + Math.cos(dir) * d, cy + Math.sin(dir) * d, r * 0.5, [
          [0, `rgba(255,200,90,${fade * 0.7})`],
          [0.6, `rgba(240,90,20,${fade * 0.35})`],
          [1, "rgba(160,30,0,0)"],
        ]);
      }
    }
    if (t < 0.5) {
      const k = t / 0.5;
      g.strokeStyle = `rgba(255,235,190,${0.8 * (1 - k)})`;
      g.lineWidth = 10 * (1 - k) + 1;
      g.beginPath();
      g.arc(cx, cy, size * (0.6 + 6 * ease(k)) * far, 0, TAU);
      g.stroke();
    }
    for (const p of sparks) {
      const k = t / p.life;
      if (k >= 1) continue;
      const drag = 1 - k * 0.45;
      const x = cx + p.vx * t * drag;
      const y = cy + p.vy * t * drag + GRAVITY * 0.35 * t * t;
      g.strokeStyle = `rgba(255,${Math.round(230 - 120 * k)},${Math.round(140 - 130 * k)},${1 - k})`;
      g.lineWidth = p.w;
      g.beginPath();
      g.moveTo(x, y);
      g.lineTo(x - p.vx * 0.03, y - p.vy * 0.03);
      g.stroke();
    }
    g.globalCompositeOperation = "source-over";
    const alpha = t < 1 ? 1 : Math.max(0, 1 - (t - 1) / 0.7);
    if (!src || alpha === 0) return;
    const sw = src.width / GRID;
    const sh = src.height / GRID;
    const swell = Math.min(1, t * 3);
    g.globalAlpha = alpha;
    for (const p of shards) {
      const grow = 1 + (p.grow - 1) * swell;
      const w = cellW * grow;
      const h = cellH * grow;
      g.save();
      g.translate(p.x + p.vx * t, p.y + p.vy * t + (GRAVITY * t * t) / 2);
      g.rotate(p.spin * t);
      g.drawImage(src, p.sx, p.sy, sw, sh, -w / 2, -h / 2, w, h);
      g.restore();
    }
    g.globalAlpha = 1;
  };

  const start = performance.now();
  let raf = 0;
  const stop = () => {
    cancelAnimationFrame(raf);
    cv.remove();
  };
  const frame = (now: number) => {
    const t = (now - start) / 1000;
    if (t >= BURST_SECONDS) {
      stop();
      done();
      return;
    }
    draw(t);
    raf = requestAnimationFrame(frame);
  };
  raf = requestAnimationFrame(frame);
  return stop;
}
