import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { Button } from "../../components/Button";
import { IconGithub, IconLink } from "../../components/glyphs";
import { QRCode } from "../../components/QRCode";
import { BrandMark, ReadmeButton } from "../../components/ReadmeButton";
import { getHealth } from "../../lib/api";
import { ANDROID_SVG, DOCKER_SVG, PLAY_SVG, UNRAID_SVG } from "../../lib/appMarks";
import { copyText } from "../../lib/clipboard";
import type { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { UnraidTileSection } from "./DashboardWidgetCard";
import { Card } from "./shared";

// The Apps page ("The App tab"): the forms of BombVault this server is not,
// then the house's companions, each with the README's buttons for the ways to
// get it and in the order of the README's rows.

const REPO = "https://github.com/junkerderprovinz/bombvault";
const APK = "bombvault-android.apk";
// The first release whose assets carry the APK.
const FIRST_APK: [number, number, number] = [9, 7, 0];

// F-Droid's mark from Simple Icons (CC0), drawn in the button's ink like the
// README's.
const FDROID_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="currentColor" d="M20.472,10.081H3.528c-0.877,0-1.589,0.711-1.589,1.589v10.59c0,0.877,0.711,1.589,1.589,1.589h16.944 c0.877,0,1.589-0.711,1.589-1.589V11.67C22.061,10.792,21.349,10.081,20.472,10.081z M12,22.525c-3.066,0-5.56-2.494-5.56-5.56 s2.494-5.56,5.56-5.56c3.066,0,5.56,2.494,5.56,5.56S15.066,22.525,12,22.525z M12,12.411c-2.511,0-4.554,2.043-4.554,4.554 S9.489,21.519,12,21.519s4.554-2.043,4.554-4.554S14.511,12.411,12,12.411z M12,20.274c-1.563,0-2.881-1.103-3.221-2.568h1.67 c0.275,0.581,0.859,0.979,1.551,0.979c0.96,0,1.721-0.761,1.721-1.721c0-0.96-0.761-1.721-1.721-1.721 c-0.649,0-1.2,0.352-1.493,0.874H8.805c0.378-1.412,1.669-2.462,3.195-2.462c1.818,0,3.309,1.491,3.309,3.309 C15.309,18.783,13.818,20.274,12,20.274z M23.849,0.396c-0.001,0.001-0.002,0.002-0.002,0.003 c-0.002-0.002-0.004-0.003-0.006-0.005c0.001-0.001,0.002-0.003,0.004-0.004c-0.116-0.137-0.279-0.231-0.519-0.238 c-0.202,0.005-0.391,0.097-0.512,0.259l-1.818,2.353c-0.164-0.058-0.339-0.095-0.523-0.095H3.528c-0.184,0-0.358,0.038-0.523,0.095 L1.187,0.41c-0.121-0.162-0.31-0.253-0.512-0.259c-0.24,0.006-0.403,0.1-0.519,0.238c0.001,0.001,0.002,0.003,0.004,0.004 C0.157,0.395,0.155,0.397,0.153,0.399C0.153,0.398,0.152,0.397,0.151,0.396C0.085,0.474-0.146,0.822,0.139,1.22l1.909,2.471 C1.981,3.867,1.94,4.057,1.94,4.257v3.707c0,0.877,0.711,1.589,1.589,1.589h16.944c0.877,0,1.589-0.711,1.589-1.589V4.257 c0-0.2-0.041-0.39-0.109-0.566l1.909-2.471C24.146,0.822,23.915,0.474,23.849,0.396z M6.904,8.228c-0.987,0-1.787-0.8-1.787-1.787 s0.8-1.787,1.787-1.787s1.787,0.8,1.787,1.787S7.891,8.228,6.904,8.228z M17.229,8.228c-0.987,0-1.787-0.8-1.787-1.787 s0.8-1.787,1.787-1.787c0.987,0,1.787,0.8,1.787,1.787S18.216,8.228,17.229,8.228z"/></svg>';

/**
 * apkRelease is the release whose APK belongs to the running server, the app
 * and the server sharing one version number, or null for a build that is no
 * release or one from before the APK was attached.
 */
export function apkRelease(version: string | null | undefined): string | null {
  const m = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(version?.trim() ?? "");
  if (!m) return null;
  const parts = m.slice(1).map(Number);
  for (let i = 0; i < 3; i++) {
    if (parts[i] !== FIRST_APK[i]) return parts[i] > FIRST_APK[i] ? parts.join(".") : null;
  }
  return parts.join(".");
}

const PARLEYPORT_REPO = "https://github.com/junkerderprovinz/parleyport";
const WIDGET_REPO = "https://github.com/junkerderprovinz/bombvault-widget";
// Community Apps gives a listing its address only once it is in the feed, and
// the search finds it under either name before and after.
const CA_SEARCH = "https://ca.unraid.net/apps?q=";
const PARLEYPORT_RUN = "docker run -d --name parleyport --restart unless-stopped -p 8760:8760 junkerderprovinz/parleyport:latest";

function open(url: string) {
  window.open(url, "_blank", "noopener,noreferrer");
}

/**
 * PhoneAppCard offers the Android app. Neither store lists it yet, so Google
 * Play and F-Droid stand as coming. The APK is the running release's own, and
 * its version stands in the card's corner; a server that is no release gets
 * the latest APK and no number, since it cannot know which one that is.
 */
export function PhoneAppCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const [release, setRelease] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    getHealth()
      .then((h) => {
        if (alive) setRelease(apkRelease(h.version));
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  const apk = release ? `${REPO}/releases/download/v${release}/${APK}` : `${REPO}/releases/latest/download/${APK}`;
  const soon = t("apps.soon");

  return (
    <Card title={t("apps.phone.title")} hint={t("apps.phone.hint")} hueIndex={hueIndex}>
      {/* On a phone the title badge reaches the top corner, so the number
          takes the bottom one. */}
      {release && (
        <a
          href={`${REPO}/releases/tag/v${release}`}
          target="_blank"
          rel="noreferrer noopener"
          className="absolute end-5 top-0 z-10 -translate-y-1/2 max-sm:top-auto max-sm:bottom-0 max-sm:translate-y-1/2 rounded-pill bg-carbon-surface2 px-2.5 py-[3px] pointer-coarse:py-[5px] font-mono text-[11px] leading-[15px] tabular-nums text-carbon-textMuted no-underline shadow-(--elevation) hover:text-carbon-text"
        >
          v{release}
        </a>
      )}
      <div className="glim-readme-btn-rows">
        <ReadmeButton tile="glim-tile-play" parts={[{ name: "Google Play" }]} mark={<BrandMark svg={PLAY_SVG} />} soonLabel={soon} />
        <ReadmeButton tile="glim-tile-fdroid" parts={[{ name: "F-Droid" }]} mark={<BrandMark svg={FDROID_SVG} />} soonLabel={soon} />
        <ApkButton t={t} href={apk} />
      </div>
    </Card>
  );
}

/**
 * ApkButton carries the APK's QR code as a segment, for a page open on a
 * computer while the file is wanted on the phone. A code cannot be scanned
 * inside a button this height, so the segment opens it in a window over the
 * button. Below the desktop width the page is on the phone already, and the
 * segment would push the unit past a narrow card, so it is left out.
 */
function ApkButton({ t, href }: { t: ReturnType<typeof useT>["t"]; href: string }) {
  const desktop = useIsDesktop();
  const [qr, setQr] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const close = useCallback(() => setQr(false), []);
  const label = t("apps.phone.qrCode");
  return (
    <div ref={box} className="inline-flex">
      <ReadmeButton
        tile="glim-tile-android"
        parts={
          desktop
            ? [
                { name: "Android", sub: "APK", href },
                { name: label, sub: "APK", onClick: () => setQr((open) => !open) },
              ]
            : [{ name: "Android", sub: "APK", href }]
        }
        mark={<BrandMark svg={ANDROID_SVG} />}
        markClass="glim-android-mark"
      />
      {qr && desktop && (
        <QrWindow anchor={box} label={label} onClose={close}>
          <QRCode value={href} size={168} />
        </QrWindow>
      )}
    </div>
  );
}

/**
 * QrWindow holds the code in a small window drawn into the body, because the
 * button's unit clips everything inside its edge. It stands above the button
 * where there is room and below it where there is not. Escape, a press
 * elsewhere, a scroll and a resize close it, since the last two carry the
 * button away from it.
 */
function QrWindow({
  anchor,
  label,
  onClose,
  children,
}: {
  anchor: RefObject<HTMLDivElement | null>;
  label: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const win = useRef<HTMLDivElement>(null);
  const [at, setAt] = useState<{ left: number; top: number } | null>(null);

  useLayoutEffect(() => {
    const box = anchor.current?.getBoundingClientRect();
    const own = win.current;
    if (!box || !own) return;
    const margin = 8;
    const { offsetWidth: width, offsetHeight: height } = own;
    const above = box.top - margin - height >= margin;
    const left = Math.max(margin, Math.min(window.innerWidth - margin - width, box.right - width));
    setAt({ left, top: above ? box.top - margin - height : box.bottom + margin });
  }, [anchor]);

  useEffect(() => {
    // A press on the segment is left to its own click, which closes the window.
    const onDown = (e: PointerEvent) => {
      const target = e.target as Node;
      if (win.current?.contains(target) || anchor.current?.contains(target)) return;
      onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("pointerdown", onDown);
    document.addEventListener("keydown", onKey);
    window.addEventListener("scroll", onClose, true);
    window.addEventListener("resize", onClose);
    return () => {
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("scroll", onClose, true);
      window.removeEventListener("resize", onClose);
    };
  }, [anchor, onClose]);

  return createPortal(
    <div
      ref={win}
      role="dialog"
      aria-label={label}
      className="glim-fade fixed z-50 overflow-hidden rounded-card shadow-(--elevation)"
      style={{ left: at?.left ?? 0, top: at?.top ?? 0, visibility: at ? "visible" : "hidden" }}
    >
      {children}
    </div>,
    document.body
  );
}

export function ParleyPortCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const navigate = useNavigate();
  const { push } = useToast();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const id = setTimeout(() => setCopied(false), 1800);
    return () => clearTimeout(id);
  }, [copied]);

  return (
    <Card title={t("apps.parleyport.title")} hint={t("apps.parleyport.hint")} hueIndex={hueIndex}>
      <div className="flex flex-wrap gap-3">
        <ReadmeButton
          tile="glim-tile-unraid"
          parts={[{ name: "Unraid", sub: t("apps.unraidSub"), href: CA_SEARCH + "parleyport" }]}
          mark={<BrandMark svg={UNRAID_SVG} />}
        />
        <ReadmeButton
          tile="glim-tile-docker"
          parts={[
            {
              name: "Docker",
              sub: copied ? t("common.copied") : t("apps.dockerSub"),
              onClick: () =>
                void copyText(PARLEYPORT_RUN).then((ok) => {
                  if (ok) setCopied(true);
                  else push(t("vm.ssh.copyFailed"), "fail");
                }),
            },
          ]}
          mark={<BrandMark svg={DOCKER_SVG} />}
          markClass="glim-docker-mark"
          note={copied}
          hint={`${t("apps.parleyport.dockerHint")} ${PARLEYPORT_RUN}`}
        />
        <ReadmeButton
          tile="glim-tile-github"
          parts={[{ name: "GitHub", sub: t("apps.repoSub"), onClick: () => open(PARLEYPORT_REPO) }]}
          mark={<IconGithub />}
          markClass="glim-github-mark"
        />
      </div>
      <Button
        label={t("apps.parleyport.toRelay")}
        labelKey="apps.parleyport.toRelay"
        glyph={<IconLink />}
        tone="neutral"
        hueIndex={hueIndex}
        onClick={() => navigate("/settings/pairing")}
        className="self-start"
      />
    </Card>
  );
}

export function WidgetAppCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  return (
    <Card title={t("apps.widget.title")} hint={t("apps.widget.hint")} hueIndex={hueIndex}>
      <div className="flex flex-wrap gap-3">
        <ReadmeButton
          tile="glim-tile-unraid"
          parts={[{ name: "Unraid", sub: t("apps.unraidSub"), href: CA_SEARCH + "bombvault%20widget" }]}
          mark={<BrandMark svg={UNRAID_SVG} />}
        />
        <ReadmeButton
          tile="glim-tile-github"
          parts={[{ name: "GitHub", sub: t("apps.repoSub"), onClick: () => open(WIDGET_REPO) }]}
          mark={<IconGithub />}
          markClass="glim-github-mark"
        />
      </div>
      <UnraidTileSection t={t} hueIndex={hueIndex} />
    </Card>
  );
}
