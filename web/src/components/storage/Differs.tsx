import { useState } from "react";
import { Link } from "react-router-dom";
import type { PlacementDomain } from "../../lib/api";
import { entryTabPath } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { placeNames, type PlacedEntry } from "../../lib/ownChoice";
import { defaultView, formatList } from "../../lib/placement";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconForward } from "../glyphs";
import { IconClose } from "../navGlyphs";

const LINK = "underline decoration-carbon-border underline-offset-[3px] hover:text-accentText";

/** Where an entry's storage locations are chosen. */
function settingsPath(entry: PlacedEntry): string {
  return entryTabPath(entry.kind, entry.item.key, "settings");
}

/** DiffersWindow lists a section's entries that back up somewhere other than
 *  its default, each with what it uses and the way to its settings. */
function DiffersWindow({ domain, entries, onClose }: { domain: PlacementDomain; entries: PlacedEntry[]; onClose: () => void }) {
  const { t, lang } = useT();
  const { options } = usePlacementOptions(domain);
  const places = (names: string[]) => (names.length > 0 ? formatList(lang, names) : null);
  const usual = options && places(placeNames(t, defaultView(options.default, options), options));

  return (
    <InstanceWindow
      title={`${t(DOMAIN_LABEL[domain])}: ${t("storage.flow.differ", entries.length)}`}
      hint={t("storage.differ.hint")}
      onClose={onClose}
      footer={<Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="accent" onClick={onClose} />}
    >
      <ul className="flex flex-col divide-y divide-carbon-border/60">
        {entries.map((entry) => {
          const uses = options && places(placeNames(t, entry.view, options));
          return (
            <li key={entry.item.key} className="relative -mx-2 flex items-center gap-3 rounded-control px-2 py-3 hover:bg-carbon-hover">
              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                {/* Stretched over the row, and in place of the window's own
                    entry, so Back from the entry returns to the page. */}
                <Link
                  to={settingsPath(entry)}
                  replace
                  className="truncate text-sm font-semibold text-carbon-text after:absolute after:inset-0 after:rounded-control focus-visible:outline-none focus-visible:after:outline-solid focus-visible:after:outline-2 focus-visible:after:outline-(--focus-ring)"
                >
                  {entry.name}
                </Link>
                {options && (
                  <span className="text-xs text-carbon-textMuted wrap-break-word">
                    {uses ? t("storage.differ.backsUpTo").replace("{places}", () => uses) : t("storage.differ.none")}
                    {usual && ` · ${t("storage.differ.default").replace("{places}", () => usual)}`}
                  </span>
                )}
              </div>
              <span className="inline-flex text-carbon-textMuted">
                <IconForward />
              </span>
            </li>
          );
        })}
      </ul>
    </InstanceWindow>
  );
}

/**
 * Differs says how many of a section's entries back up somewhere other than
 * its default. It leads to the one entry's settings, or opens the list of
 * them where there are several.
 */
export function Differs({ domain, entries }: { domain: PlacementDomain; entries: PlacedEntry[] }) {
  const { t } = useT();
  const [open, setOpen] = useState(false);
  if (entries.length === 0) return null;
  const label = t("storage.flow.differ", entries.length);
  if (entries.length === 1) {
    return (
      <Link data-new="storage-flow" to={settingsPath(entries[0])} className={LINK}>
        {label}
      </Link>
    );
  }
  return (
    <>
      <button type="button" data-new="storage-flow" onClick={() => setOpen(true)} className={`cursor-pointer ${LINK}`}>
        {label}
      </button>
      {open && <DiffersWindow domain={domain} entries={entries} onClose={() => setOpen(false)} />}
    </>
  );
}
