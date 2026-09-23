import qrcode from "qrcode-generator";
import { useMemo } from "react";

// An otpauth:// URI as a scannable QR code, drawn as one SVG path rather than a
// <rect> per module: a typical code has around a thousand dark modules.
//
// Black on white in every theme, because many scanners cannot read an inverted
// code. The white margin is the quiet zone the spec requires.

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
