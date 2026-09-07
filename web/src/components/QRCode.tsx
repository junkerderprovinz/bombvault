import qrcode from "qrcode-generator";
import { useMemo } from "react";

// ---------------------------------------------------------------------------
// QRCode — an otpauth:// URI as a scannable square.
//
// Drawn as ONE SVG path rather than a grid of <rect>s. A typical otpauth URI
// lands on a 33x33 module code, which is around a thousand dark modules; as
// elements that is a thousand DOM nodes for a picture, and as a path it is one.
//
// The colours are fixed black on white and do NOT follow the theme, which is
// deliberate. A phone camera needs contrast in the direction it expects, and a
// code drawn in the interface's own dark surface with a light foreground is
// inverted: some scanners cope, plenty do not, and "my authenticator will not
// read it" is a bad first minute with a security feature. The white square is
// given a little padding of its own because the quiet zone is part of the spec,
// not decoration.
// ---------------------------------------------------------------------------

export function QRCode({
  value,
  size = 200,
  className,
}: {
  value: string;
  /** Rendered edge length in pixels. The SVG scales, so this is presentation only. */
  size?: number;
  className?: string;
}) {
  const { path, extent } = useMemo(() => build(value), [value]);
  return (
    <svg
      viewBox={`0 0 ${extent} ${extent}`}
      width={size}
      height={size}
      className={className}
      role="img"
      aria-hidden="true"
      shapeRendering="crispEdges"
    >
      <rect width={extent} height={extent} fill="#ffffff" />
      <path d={path} fill="#000000" />
    </svg>
  );
}

/** QUIET is the mandatory clear margin around a code, in modules. */
const QUIET = 4;

function build(value: string): { path: string; extent: number } {
  // Type 0 means "pick the smallest version that fits"; level M is the usual
  // trade for a screen, where the code is not going to be smudged or folded.
  const qr = qrcode(0, "M");
  qr.addData(value);
  qr.make();
  const count = qr.getModuleCount();

  const parts: string[] = [];
  for (let row = 0; row < count; row++) {
    // Runs of adjacent dark modules become one rectangle instead of one each.
    let runStart = -1;
    for (let col = 0; col <= count; col++) {
      const dark = col < count && qr.isDark(row, col);
      if (dark && runStart < 0) {
        runStart = col;
      } else if (!dark && runStart >= 0) {
        parts.push(`M${runStart + QUIET} ${row + QUIET}h${col - runStart}v1h-${col - runStart}z`);
        runStart = -1;
      }
    }
  }
  return { path: parts.join(""), extent: count + QUIET * 2 };
}
