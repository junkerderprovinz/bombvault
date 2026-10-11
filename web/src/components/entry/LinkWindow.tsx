import { useState } from "react";
import { useT } from "../../lib/i18n";
import { countBackups, useTakeOver, type TakeoverEntry } from "../../lib/useTakeOver";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconLink } from "../glyphs";
import { Rows } from "../storage/rows";

/**
 * LinkWindow lists the entries that are not installed, for the one the
 * entry ran as before it was renamed. Taking one over moves its backups and
 * settings here.
 */
export function LinkWindow({
  candidates,
  entry,
  onDone,
  onClose,
}: {
  /** The not-installed entries of the same kind, by the names their routes take. */
  candidates: string[];
  entry: TakeoverEntry;
  onDone: () => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const { takeOver, busy, confirmDialog } = useTakeOver(entry, onDone, t);
  const [counting, setCounting] = useState(false);

  async function link(from: string) {
    setCounting(true);
    const backups = await countBackups(entry.api, from);
    setCounting(false);
    await takeOver(from, backups, () => undefined);
  }

  return (
    <InstanceWindow
      title={t("entry.menu.link")}
      hint={t("takeover.linkHint")}
      facts={t("entry.link.for").replace("{name}", () => entry.displayName)}
      onClose={onClose}
      footer={<Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />}
    >
      <Rows>
        {candidates.map((from) => (
          <div key={from} className="flex flex-wrap items-center justify-between gap-3 py-3 first:pt-0 last:pb-0">
            <span className="min-w-0 text-sm font-semibold text-carbon-text wrap-anywhere">{from}</span>
            <Button
              label={t("takeover.accept")}
              labelKey="takeover.accept"
              glyph={<IconLink />}
              disabled={busy || counting}
              onClick={() => void link(from)}
            />
          </div>
        ))}
      </Rows>
      {confirmDialog}
    </InstanceWindow>
  );
}
