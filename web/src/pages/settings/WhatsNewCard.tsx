import { useNavigate } from "react-router-dom";

import { Button } from "../../components/Button";
import { IconEye, IconForward } from "../../components/glyphs";
import { IconCheck } from "../../components/navGlyphs";
import { useT } from "../../lib/i18n";
import { CHANGES, type Change } from "../../lib/whatsNew";
import { markSeen, showDots, useWhatsNew } from "../../lib/whatsNewStore";
import { Card, ToggleRow } from "./shared";

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
 * WhatsNewCard lists what the last update changed, page by page, with the way
 * to each change. It also holds the switch for the dots that mark the changes
 * on their pages, and counts the ones nobody has clicked away yet.
 */
export function WhatsNewCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const navigate = useNavigate();
  const { on, seen, absent, unseen } = useWhatsNew();

  return (
    <Card title={t("whatsNew.title")} hint={t("whatsNew.hint")} hueIndex={hueIndex}>
      <ToggleRow label={t("whatsNew.show")} checked={on} onChange={showDots} />
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <span data-testid="whats-new-left" className="text-sm tabular-nums text-carbon-textSub">
          {t("whatsNew.count").replace("{left}", String(unseen.length)).replace("{all}", String(CHANGES.length))}
        </span>
        <span className="flex min-w-0 flex-wrap items-center justify-end gap-2">
          <Button
            label={t("whatsNew.showAll")}
            labelKey="whatsNew.showAll"
            glyph={<IconEye />}
            onClick={() => markSeen(ALL, false)}
            disabled={seen.size === 0 && absent.size === 0}
            className="glim-btn-wrap"
          />
          <Button
            label={t("whatsNew.markAll")}
            labelKey="whatsNew.markAll"
            glyph={<IconCheck />}
            onClick={() => markSeen(ALL)}
            disabled={unseen.length === 0}
            className="glim-btn-wrap"
          />
        </span>
      </div>
      {byPage(t).map((page) => (
        <section key={page.name} className="flex flex-col gap-1.5">
          <h3 className="text-caption font-semibold uppercase tracking-[.12em] text-carbon-textMuted">{page.name}</h3>
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
                    onClick={() => navigate(change.go ?? change.page)}
                    className="ms-auto glim-btn-wrap"
                  />
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </Card>
  );
}
