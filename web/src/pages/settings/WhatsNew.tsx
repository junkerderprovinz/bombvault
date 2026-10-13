import { useNavigate } from "react-router-dom";

import { Button } from "../../components/Button";
import { IconEye, IconForward } from "../../components/glyphs";
import { IconCheck } from "../../components/navGlyphs";
import { useT } from "../../lib/i18n";
import { CHANGES, type Change } from "../../lib/whatsNew";
import { markSeen, useWhatsNew } from "../../lib/whatsNewStore";
import { Card } from "./shared";

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
 * to each change. One button in its heading marks everything as seen while
 * something is left, which also hides the dots, and shows everything again
 * once nothing is. It counts what it would mark.
 */
export function WhatsNewCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const navigate = useNavigate();
  const { seen, absent, unseen } = useWhatsNew();
  const left = unseen.length > 0;

  return (
    <Card
      title={t("whatsNew.title")}
      hint={t("whatsNew.hint")}
      hueIndex={hueIndex}
      action={
        <Button
          label={`${t(left ? "whatsNew.markAll" : "whatsNew.showAll")} · ${left ? unseen.length : CHANGES.length}`}
          labelKey={left ? "whatsNew.markAll" : "whatsNew.showAll"}
          glyph={left ? <IconCheck /> : <IconEye />}
          onClick={() => markSeen(ALL, left)}
          className="glim-btn-wrap"
        />
      }
    >
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
