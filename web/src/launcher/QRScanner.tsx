import jsQR from "jsqr";
import { useEffect, useRef } from "react";
import { Button } from "../components/Button";
import { IconCancel } from "../components/glyphs";
import { IconCheckCircle } from "../components/navGlyphs";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

/** How often a frame is read. A code held still is found within a few. */
const READ_MS = 150;
/** An empty picture for the video until the first frame, where a WebView
 *  would otherwise draw its own play button. */
const NO_POSTER = "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7";
/** Frames are scaled down to this width before reading, which keeps the
 *  reading fast on an older phone and is plenty for a code filling the frame. */
const READ_WIDTH = 640;

/**
 * mainBackCamera picks the back camera Android numbers first, which is the
 * main one. Asking for the environment camera alone can land on a telephoto
 * lens that focuses no nearer than half a metre. Labels read like
 * `camera2 0, facing back`.
 */
export function mainBackCamera(devices: MediaDeviceInfo[]): string | undefined {
  return devices
    .filter((d) => d.kind === "videoinput" && /back|environment/i.test(d.label))
    .map((d) => ({ id: d.deviceId, n: Number(/(\d+)\s*,\s*facing/i.exec(d.label)?.[1] ?? Infinity) }))
    .sort((a, b) => a.n - b.n)[0]?.id;
}

/** Opens the main back camera. The labels are known only once a camera is open. */
async function openMainBackCamera(): Promise<MediaStream> {
  const first = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" }, audio: false });
  const main = mainBackCamera(await navigator.mediaDevices.enumerateDevices());
  if (!main || first.getVideoTracks()[0]?.getSettings().deviceId === main) return first;
  first.getTracks().forEach((track) => track.stop());
  return navigator.mediaDevices.getUserMedia({ video: { deviceId: { exact: main } }, audio: false });
}

/**
 * QRScanner is a window over the whole screen that hands back the first QR
 * code the camera sees. The camera picture fills it, an accent frame shows
 * where to hold the code, and Cancel sits at the end of the bottom row like
 * every window's footer.
 */
export function QRScanner({
  t,
  hint,
  camera,
  onAskCamera,
  onScanned,
  onClose,
}: {
  t: T;
  hint: string;
  /** Whether the app may use the camera; without it the window asks first. */
  camera: boolean;
  onAskCamera: () => void;
  onScanned: (text: string) => void;
  onClose: () => void;
}) {
  const video = useRef<HTMLVideoElement>(null);
  // The latest callbacks, so a new render does not reopen the camera.
  const scanned = useRef(onScanned);
  const refused = useRef(onAskCamera);
  scanned.current = onScanned;
  refused.current = onAskCamera;

  useEffect(() => {
    if (!camera) return;
    let stream: MediaStream | null = null;
    let timer = 0;
    let done = false;
    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("2d", { willReadFrequently: true });

    function read() {
      const v = video.current;
      if (done || !v || !ctx || v.videoWidth === 0) return;
      canvas.width = Math.min(READ_WIDTH, v.videoWidth);
      canvas.height = Math.round((v.videoHeight * canvas.width) / v.videoWidth);
      ctx.drawImage(v, 0, 0, canvas.width, canvas.height);
      const code = jsQR(ctx.getImageData(0, 0, canvas.width, canvas.height).data, canvas.width, canvas.height);
      if (code?.data) {
        // One code per opening, or a code held still would be reported on
        // every frame.
        done = true;
        scanned.current(code.data);
      }
    }

    openMainBackCamera()
      .then((s) => {
        stream = s;
        if (done) return;
        if (video.current) {
          video.current.srcObject = s;
          void video.current.play();
        }
        timer = window.setInterval(read, READ_MS);
      })
      .catch(() => refused.current());

    return () => {
      done = true;
      window.clearInterval(timer);
      stream?.getTracks().forEach((track) => track.stop());
    };
  }, [camera]);

  return (
    <div role="dialog" aria-modal="true" aria-label={hint} className="glim-modal-backdrop fixed inset-0 z-50">
      {camera ? (
        <>
          <video ref={video} muted playsInline poster={NO_POSTER} className="absolute inset-0 h-full w-full object-cover" />
          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-6">
            <div className="h-60 w-60 rounded-card border-2 border-accent" />
            <p className="px-8 text-center text-sm text-carbon-text">{hint}</p>
          </div>
        </>
      ) : (
        <div className="flex h-full flex-col items-center justify-center gap-4 p-6">
          <p className="text-center text-sm text-carbon-text">{t("launcher.cameraHint")}</p>
          <Button label={t("launcher.cameraAllow")} labelKey="launcher.cameraAllow" glyph={<IconCheckCircle />} tone="accent" onClick={onAskCamera} />
        </div>
      )}
      {/* Quiet, because leaving is not what this window is for. */}
      <div className="absolute inset-x-6 bottom-10 flex justify-end">
        <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconCancel />} tone="neutral" onClick={onClose} />
      </div>
    </div>
  );
}
