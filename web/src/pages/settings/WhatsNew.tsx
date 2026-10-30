import { useEffect, useId, useMemo, useState, type CSSProperties } from "react";
import { useNavigate } from "react-router-dom";

import { Badge } from "../../components/Badge";
import { hueVars } from "../../lib/appearance";
import { Button } from "../../components/Button";
import { IconConfirm, IconEye, IconForward, IconWhatsNew } from "../../components/glyphs";
import { IconClose } from "../../components/navGlyphs";
import { ReleaseNotes, releasePage } from "../../components/ReleaseNotes";
import { useT } from "../../lib/i18n";
import { CHANGES, placed, RELEASES, type Change } from "../../lib/whatsNew";
import { markSeen, newer, openWhatsNew, releaseOf, useWhatsNew, useWhatsNewOpen } from "../../lib/whatsNewStore";
import { InstanceWindow } from "../instances/InstanceWindow";

type T = ReturnType<typeof useT>["t"];

/** The changes of a release page by page, in the order the list names the
 *  pages. Those no page shows come first. */
function byPage(t: T, changes: Change[]): { name: string; changes: Change[] }[] {
  const pages: { name: string; changes: Change[] }[] = [];
  for (const change of changes) {
    const name = placed(change) ? change.where.map((key) => t(key)).join(" › ") : t("whatsNew.page.none");
    const page = pages.find((p) => p.name === name);
    if (page) page.changes.push(change);
    else if (placed(change)) pages.push({ name, changes: [change] });
    else pages.unshift({ name, changes: [change] });
  }
  return pages;
}

/**
 * The releases the window shows a card for, newest first: every release since
 * the one this browser saw before the last update, any release with a change
 * still unseen, and the running one. The server knows the releases that
 * brought nothing for the list; without its answer the list's own releases
 * stand in.
 */
function releasesShown(running: string | null, from: string | null, unseen: Change[], fetched: string[] | null): string[] {
  const shown = new Set<string>(unseen.map((change) => change.since));
  if (running) shown.add(running);
  const since = fetched ?? (from ? RELEASES.map((r) => r.version).filter((v) => newer(v, from) && (!running || !newer(v, running))) : []);
  for (const version of since) shown.add(version);
  return [...shown].sort((a, b) => (newer(a, b) ? -1 : newer(b, a) ? 1 : 0));
}

function useReleasesSince(from: string | null): string[] | null {
  const [fetched, setFetched] = useState<{ from: string; versions: string[] } | null>(null);
  useEffect(() => {
    if (!from) return;
    let active = true;
    fetch(`/api/release-notes/versions?from=v${encodeURIComponent(from)}`, { headers: { Accept: "application/json" } })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then((data: { ok?: boolean; versions?: string[] }) => {
        if (active && data.ok && Array.isArray(data.versions)) {
          setFetched({ from, versions: data.versions.map((tag) => tag.replace(/^v/, "")) });
        }
      })
      .catch(() => {
        // The list's own releases stand in.
      });
    return () => {
      active = false;
    };
  }, [from]);
  return fetched && fetched.from === from ? fetched.versions : null;
}

function ChangeRow({ change, seen, absent, describedBy, onGo }: { change: Change; seen: boolean; absent: boolean; describedBy: string; onGo: (to: string) => void }) {
  const { t } = useT();
  return (
    <li
      data-change={change.id}
      data-seen={seen || undefined}
      data-absent={absent || undefined}
      className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-control bg-carbon-surface px-3 py-2.5"
    >
      <i aria-hidden="true" className={`glim-new-dot glim-new-dot-still${seen ? " glim-new-dot-seen" : ""}`} />
      {/* A seen change is greyed in its words, which keep their contrast, and
          an unseen one says so to a screen reader. */}
      <span
        id={describedBy}
        className={`flex min-w-0 flex-[1_1_9rem] flex-col gap-0.5 text-sm wrap-break-word ${seen ? "text-carbon-textSub" : "text-carbon-text"}`}
      >
        {!seen && <span className="sr-only">{t("whatsNew.title")}: </span>}
        {t(change.text)}
        {absent && <span className="text-xs text-carbon-textSub">{t("whatsNew.absent")}</span>}
      </span>
      {placed(change) && (
        <Button
          label={t("whatsNew.go")}
          labelKey="whatsNew.go"
          glyph={<IconForward />}
          ariaDescribedBy={describedBy}
          onClick={() => onGo(change.go ?? change.page)}
          className="ms-auto glim-btn-wrap"
        />
      )}
    </li>
  );
}

/** One release as a card of its own: its version on the badge, its notes,
 *  and its changes. */
function ReleaseCard({ version, hueIndex, onGo }: { version: string; hueIndex: number; onGo: (to: string) => void }) {
  const { t } = useT();
  const { seen, absent } = useWhatsNew();
  const id = useId();
  const changes = CHANGES.filter((change) => change.since === version);

  return (
    <section
      data-release={version}
      aria-labelledby={`${id}-v`}
      className="relative glim-notch-card glim-hue flex flex-col gap-4 rounded-card bg-carbon-surface2 p-4 sm:p-5"
      style={hueVars(hueIndex) as CSSProperties}
    >
      <h3 id={`${id}-v`} className="flex items-center">
        <Badge tone="heading" size="heading" hueIndex={hueIndex}>
          {version}
        </Badge>
      </h3>
      <ReleaseNotes version={`v${version}`} />
      {byPage(t, changes).map((page) => (
        <div key={page.name} className="flex flex-col gap-1.5">
          <h4 className="glim-eyebrow">{page.name}</h4>
          <ul className="flex flex-col gap-1.5">
            {page.changes.map((change) => (
              <ChangeRow
                key={change.id}
                change={change}
                seen={seen.has(change.id)}
                absent={absent.has(change.id)}
                describedBy={`${id}-${change.id}`}
                onGo={onGo}
              />
            ))}
          </ul>
        </div>
      ))}
    </section>
  );
}

/**
 * WhatsNewWindow is what the updates brought, one card per release since the
 * one this browser saw before. One button marks every listed change as seen
 * while something is left, which also hides the dots, and shows them all
 * again once nothing is. A change no page shows counts as seen once the
 * window that listed it closes. `version` is the running release tag, null
 * in a build without one.
 */
export function WhatsNewWindow({ version, onClose }: { version: string | null; onClose: () => void }) {
  const { t } = useT();
  const navigate = useNavigate();
  const { seen, unseen, from } = useWhatsNew();
  const running = releaseOf(version ?? "");
  const fetched = useReleasesSince(from);
  const releases = useMemo(() => releasesShown(running, from, unseen, fetched), [running, from, unseen, fetched]);
  const listed = CHANGES.filter((change) => releases.includes(change.since));
  const left = listed.filter((change) => !seen.has(change.id));

  const close = () => {
    const read = left.filter((change) => !placed(change)).map((change) => change.id);
    if (read.length > 0) markSeen(read);
    onClose();
  };
  const go = (to: string) => {
    close();
    navigate(to, { replace: true });
  };

  const title =
    from && running && newer(running, from)
      ? t("whatsNew.windowTitleSince").replace("{version}", from)
      : t("whatsNew.windowTitle").replace("{version}", running ?? RELEASES[0]?.version ?? "");

  return (
    <InstanceWindow
      title={title}
      hint={t("whatsNew.hint")}
      onClose={close}
      footer={
        <>
          <Badge as="a" href={releasePage(version)} target="_blank" rel="noopener noreferrer" tone="neutral" size="large" className="me-auto">
            {t("whatsnew.viewOnGitHub")}
          </Badge>
          <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="accent" onClick={close} />
        </>
      }
    >
      <div className="flex flex-col gap-6">
        {listed.length > 0 && (
          <div className="flex justify-end">
            <Button
              label={`${t(left.length > 0 ? "whatsNew.markAll" : "whatsNew.showAll")} · ${left.length > 0 ? left.length : listed.length}`}
              labelKey={left.length > 0 ? "whatsNew.markAll" : "whatsNew.showAll"}
              glyph={left.length > 0 ? <IconConfirm /> : <IconEye />}
              tone="neutral"
              onClick={() => markSeen(listed.map((change) => change.id), left.length > 0)}
              className="glim-btn-wrap"
            />
          </div>
        )}
        {releases.length > 0 ? (
          releases.map((release, i) => <ReleaseCard key={release} version={release} hueIndex={i} onGo={go} />)
        ) : (
          <ReleaseNotes version={version} />
        )}
      </div>
    </InstanceWindow>
  );
}

/** WhatsNewHost draws the window wherever it was opened from, once in the
 *  app's layout. */
export function WhatsNewHost({ version }: { version: string | null }) {
  if (!useWhatsNewOpen()) return null;
  return <WhatsNewWindow version={version} onClose={() => openWhatsNew(false)} />;
}

/**
 * WhatsNewButton stands in the Version card and opens the window. It wears
 * the gift like every button wears its glyph, counts what is still unseen,
 * and the dot before the gift goes grey once nothing is.
 */
export function WhatsNewButton() {
  const { t } = useT();
  const left = useWhatsNew().unseen.length;
  return (
    <Button
      label={left > 0 ? `${t("whatsNew.title")} · ${left}` : t("whatsNew.title")}
      labelKey={null}
      glyph={
        <span className="inline-flex items-center gap-1.5">
          <i aria-hidden="true" className={`glim-new-dot glim-new-dot-still${left > 0 ? "" : " glim-new-dot-seen"}`} />
          <IconWhatsNew />
        </span>
      }
      keepLabel
      onClick={() => openWhatsNew(true)}
    />
  );
}
