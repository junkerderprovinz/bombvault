import { useNavigate } from "react-router-dom";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconEye, IconForward } from "../../components/glyphs";
import { IconCheck, IconClose } from "../../components/navGlyphs";
import { ReleaseNotes, releasePage } from "../../components/ReleaseNotes";
import { Selector } from "../../components/Selector";
import { useT } from "../../lib/i18n";
import { CHANGES, WHATS_NEW_RELEASE, type Change } from "../../lib/whatsNew";
import { markSeen, openWhatsNew, useWhatsNew, useWhatsNewOpen, type NewsTab } from "../../lib/whatsNewStore";
import { InstanceWindow } from "../instances/InstanceWindow";

const ALL = CHANGES.map((change) => change.id);

/** The changes of each page together, in the order the list names the pages. */
function byPage(t: ReturnType<typeof useT>["t"]): { name: string; changes: Change[] }[] {
  const pages: { name: string; changes: Change[] }[] = [];
  for (const change of CHANGES) {
    const name = change.where.map((key) => t(key)).join(" › ");
    const page = pages.find((p) => p.name === name);
    if (page) page.changes.push(change);
    else pages.push({ name, changes: [change] });
  }
  return pages;
}

/**
 * WhatsNewWindow is what the last update brought, on two tabs: the release
 * notes of the running version, and the list of changes page by page with the
 * way to each. On the list one button marks everything as seen while
 * something is left, which also hides the dots, and shows everything again
 * once nothing is. It counts what it would mark. `version` is the running
 * release tag, null in a build without one.
 */
export function WhatsNewWindow({
  tab,
  version,
  onTab,
  onClose,
}: {
  tab: NewsTab;
  version: string | null;
  onTab: (tab: NewsTab) => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const navigate = useNavigate();
  const { seen, absent, unseen } = useWhatsNew();
  const left = unseen.length > 0;
  const listed = CHANGES.length > 0;
  const shown = listed ? tab : "notes";
  const release = version?.replace(/^v/, "") || WHATS_NEW_RELEASE;

  return (
    <InstanceWindow
      title={t("whatsNew.windowTitle").replace("{version}", release)}
      hint={t("whatsNew.hint")}
      onClose={onClose}
      footer={
        <>
          <Badge as="a" href={releasePage(version)} target="_blank" rel="noopener noreferrer" tone="neutral" size="large" className="me-auto">
            {t("whatsnew.viewOnGitHub")}
          </Badge>
          <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="accent" onClick={onClose} />
        </>
      }
    >
      {listed && (
        <Selector
          items={[
            { id: "notes", label: t("whatsNew.notesTab") },
            { id: "changes", label: left ? `${t("whatsNew.changesTab")} · ${unseen.length}` : t("whatsNew.changesTab") },
          ]}
          label={t("whatsNew.windowTitle").replace("{version}", release)}
          select="one"
          active={shown}
          onChange={(id) => onTab(id as NewsTab)}
          fit
          buttonHeight
        />
      )}
      {shown === "notes" ? (
        <ReleaseNotes version={version} />
      ) : (
        <>
          <div className="flex justify-end">
            <Button
              label={`${t(left ? "whatsNew.markAll" : "whatsNew.showAll")} · ${left ? unseen.length : CHANGES.length}`}
              labelKey={left ? "whatsNew.markAll" : "whatsNew.showAll"}
              glyph={left ? <IconCheck /> : <IconEye />}
              tone="neutral"
              onClick={() => markSeen(ALL, left)}
              className="glim-btn-wrap"
            />
          </div>
          {byPage(t).map((page) => (
            <section key={page.name} className="flex flex-col gap-1.5">
              <h3 className="glim-eyebrow">{page.name}</h3>
              <ul className="flex flex-col gap-1.5">
                {page.changes.map((change) => {
                  const done = seen.has(change.id);
                  const away = absent.has(change.id);
                  return (
                    <li
                      key={change.id}
                      data-change={change.id}
                      data-seen={done || undefined}
                      data-absent={away || undefined}
                      className={`flex flex-wrap items-center gap-x-3 gap-y-2 rounded-control bg-carbon-surface2 px-3 py-2.5${done ? " opacity-55" : ""}`}
                    >
                      <i aria-hidden="true" className={`glim-new-dot glim-new-dot-still${done ? " glim-new-dot-seen" : ""}`} />
                      <span className="flex min-w-0 flex-[1_1_9rem] flex-col gap-0.5 text-sm text-carbon-text wrap-break-word">
                        {t(change.text)}
                        {away && <span className="text-xs text-carbon-textSub">{t("whatsNew.absent")}</span>}
                      </span>
                      <Button
                        label={t("whatsNew.go")}
                        labelKey="whatsNew.go"
                        glyph={<IconForward />}
                        onClick={() => {
                          onClose();
                          navigate(change.go ?? change.page, { replace: true });
                        }}
                        className="ms-auto glim-btn-wrap"
                      />
                    </li>
                  );
                })}
              </ul>
            </section>
          ))}
        </>
      )}
    </InstanceWindow>
  );
}

/** WhatsNewHost draws the window wherever it was opened from, once in the
 *  app's layout. */
export function WhatsNewHost({ version }: { version: string | null }) {
  const tab = useWhatsNewOpen();
  if (!tab) return null;
  return <WhatsNewWindow tab={tab} version={version} onTab={openWhatsNew} onClose={() => openWhatsNew(null)} />;
}

/** WhatsNewButton stands in the Version card and opens the list of changes.
 *  It counts what is still unseen, and its dot goes grey once nothing is. */
export function WhatsNewButton() {
  const { t } = useT();
  const { unseen } = useWhatsNew();
  if (CHANGES.length === 0) return null;
  const left = unseen.length;
  return (
    <Button
      label={left > 0 ? `${t("whatsNew.title")} · ${left}` : t("whatsNew.title")}
      labelKey={null}
      glyph={<i aria-hidden="true" className={`glim-new-dot glim-new-dot-still${left > 0 ? "" : " glim-new-dot-seen"}`} />}
      keepLabel
      onClick={() => openWhatsNew("changes")}
    />
  );
}
