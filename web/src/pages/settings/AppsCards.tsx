import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { Button } from "../../components/Button";
import { IconGithub, IconLink } from "../../components/glyphs";
import { IconLive } from "../../components/navGlyphs";
import { QRCode } from "../../components/QRCode";
import { BrandMark, ReadmeButton } from "../../components/ReadmeButton";
import { getHealth } from "../../lib/api";
import { ANDROID_SVG, DOCKER_SVG, PLAY_SVG, UNRAID_SVG } from "../../lib/appMarks";
import { copyText } from "../../lib/clipboard";
import type { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { Card } from "./shared";

// The Apps page ("The App tab"): the forms of BombVault this server is not,
// then the house's companions, each with the README's buttons for the ways to
// get it and in the order of the README's rows.

const REPO = "https://github.com/junkerderprovinz/bombvault";
const APK = "bombvault-android.apk";
const FDROID = "https://f-droid.org/packages/bombvault.halleluja.design/";
// The first release whose assets carry the APK.
const FIRST_APK: [number, number, number] = [9, 7, 0];

// F-Droid's 2015 logo from its artwork repository (gitlab.com/fdroid/artwork,
// CC BY-SA 3.0 or GPL-3.0-or-later), in its own colours. The eyes are holes, so
// the tile shows through them. The ring and the F lie on the body, so under the
// pointer they take the tile colour through --mark-cut.
const FDROID_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="-0.05 -0.52 48.1 48.1"><path fill="none" stroke="var(--mark-ink, #769616)" stroke-width="2.5" stroke-linecap="round" d="M2.589 2.5 6.839 8M45.411 2.5 41.161 8"/><path fill="var(--mark-ink, #aeea00)" fill-rule="evenodd" d="M8 6h32a3 3 0 0 1 3 3v7a3 3 0 0 1-3 3H8a3 3 0 0 1-3-3V9a3 3 0 0 1 3-3zm3 7.125a3.375 3.375 0 1 0 6.75 0 3.375 3.375 0 1 0-6.75 0zm19.5 0a3.375 3.375 0 1 0 6.75 0 3.375 3.375 0 1 0-6.75 0z"/><rect x="5" y="20" width="38" height="26" rx="3" fill="var(--mark-ink, #1976d2)"/><circle cx="24" cy="33" r="9.55" fill="none" stroke="var(--mark-cut, #0d47a1)" stroke-width="1.9"/><path fill="var(--mark-cut, #0d47a1)" d="m 24,26.75c-2.880662,0 -5.319789,1.984685 -6.033203,4.650391l3.212891,0C21.734004,30.415044 22.774798,29.75 24,29.75c1.812692,0 3.25,1.437308 3.25,3.25 0,1.812693 -1.437308,3.25 -3.25,3.25 -1.307381,0 -2.411251,-0.75269 -2.929688,-1.849609l-3.154296,0C18.558263,37.166146 21.04791,39.25 24,39.25c3.434013,0 6.25,-2.815987 6.25,-6.25 0,-3.434012 -2.815987,-6.25 -6.25,-6.25 z"/></svg>';

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
 * PhoneAppCard offers the Android app. F-Droid lists it and Google Play does
 * not yet, so Play stands as coming. The APK is the running release's own, and
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
          className="absolute end-5 top-0 z-10 -translate-y-1/2 max-sm:top-auto max-sm:bottom-0 max-sm:translate-y-1/2 rounded-pill bg-carbon-surface2 px-2.5 py-[3px] pointer-coarse:py-[5px] font-mono text-meta leading-[15px] tabular-nums text-carbon-textMuted no-underline shadow-(--elevation) hover:text-carbon-text"
        >
          v{release}
        </a>
      )}
      <div className="glim-readme-btn-rows">
        <ReadmeButton tile="glim-tile-play" parts={[{ name: "Google Play" }]} mark={<BrandMark svg={PLAY_SVG} />} soonLabel={soon} />
        <ReadmeButton
          tile="glim-tile-fdroid"
          parts={[{ name: "F-Droid", href: FDROID }]}
          mark={<BrandMark svg={FDROID_SVG} />}
          soonLabel={soon}
          newId="apps-fdroid"
        />
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
    <Card
      title={t("apps.parleyport.title")}
      hint={t("apps.parleyport.hint")}
      hueIndex={hueIndex}
      foot={
        <Button
          label={t("apps.parleyport.toRelay")}
          labelKey="apps.parleyport.toRelay"
          glyph={<IconLink />}
          tone="neutral"
          hueIndex={hueIndex}
          onClick={() => navigate("/settings/pairing")}
        />
      }
    >
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
    </Card>
  );
}

export function WidgetAppCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const navigate = useNavigate();
  return (
    <Card
      title={t("apps.widget.title")}
      hint={t("apps.widget.cardHint")}
      hueIndex={hueIndex}
      foot={
        <Button
          label={t("apps.widget.toSettings")}
          labelKey="apps.widget.toSettings"
          glyph={<IconLive />}
          tone="neutral"
          hueIndex={hueIndex}
          onClick={() => navigate("/settings/integrations")}
        />
      }
    >
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
    </Card>
  );
}
