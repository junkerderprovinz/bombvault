import { useState, type CSSProperties } from "react";
import { Link } from "react-router-dom";
import {
  PLACEMENT_DOMAINS,
  previewItemPlacement,
  setItemPlacement,
  type DefaultRow,
  type DroppedTarget,
  type OkEnvelope,
  type PlacementOptions,
  type UploadEstimate,
} from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { entryPath } from "../../lib/backupEntry";
import { useT } from "../../lib/i18n";
import { followChange, placeNames, type PlacedEntry } from "../../lib/ownChoice";
import { defaultView, formatList, lockHint, viewHomeLabel } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useHostLabel } from "../../lib/useHostLabel";
import { droppedText } from "../../lib/usePlacementSave";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { Card } from "../../pages/settings/shared";
import { Button } from "../Button";
import { IconRefresh } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { UploadLines } from "../placement/PlacementRow";
import { DomainGlyph } from "./rows";

type Preview = OkEnvelope & { added?: UploadEstimate[]; dropped?: DroppedTarget[] };

/**
 * OwnChoiceCard lists the entries that are backed up somewhere other than
 * their section, each with the way back to the section's default. The caller
 * shows it only while there is such an entry.
 */
export function OwnChoiceCard({
  entries,
  defaults,
  hueIndex,
}: {
  entries: PlacedEntry[];
  defaults: DefaultRow[];
  hueIndex: number;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const host = useHostLabel();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState(false);
  const options: Record<string, PlacementOptions | null> = {
    containers: usePlacementOptions("containers").options,
    vms: usePlacementOptions("vms").options,
    files: usePlacementOptions("files").options,
  };

  const places = (names: string[]) => (names.length > 0 ? formatList(lang, names) : null);

  async function follow(entry: PlacedEntry, row: DefaultRow, offered: PlacementOptions) {
    const change = followChange(entry.view);
    if (!change) return;
    setBusy(true);
    try {
      const preview: Preview = await previewItemPlacement(entry.item, change).catch((e: unknown) => ({
        ok: false,
        error: e instanceof Error ? e.message : undefined,
      }));
      const after = entry.view.locked ? { ...entry.view, skip: row.skip } : defaultView(row, offered);
      const goes = places(placeNames(t, after, offered)) ?? host;
      const question = t("storage.choice.ask").replace("{name}", () => entry.name).replace("{places}", () => goes);
      const added = (preview.added ?? []).filter((a) => a.snapshots > 0);
      const extra = (
        <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
          {!preview.ok && <p>{placementErrorText(t, lang, preview, "settings.error")}</p>}
          {(preview.dropped ?? []).map((d) => (
            <p key={d.targetId}>{droppedText(t, d)}</p>
          ))}
          {added.length > 0 && <UploadLines added={added} />}
        </div>
      );
      const yes = await confirm(question, {
        extra,
        confirmLabel: t("storage.choice.follow"),
        confirmLabelKey: "storage.choice.follow",
        cancelTone: "neutral",
      });
      if (!yes) return;
      const res = await setItemPlacement(entry.item, change).catch(
        (e: unknown): OkEnvelope => ({ ok: false, error: e instanceof Error ? e.message : undefined })
      );
      if (!res.ok) {
        push(placementErrorText(t, lang, res, "settings.error"), "fail");
        return;
      }
      push(t("storage.choice.followed").replace("{name}", () => entry.name), "success");
      placementChanged();
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title={t("storage.choice.title")} hint={t("storage.choice.hint")} hueIndex={hueIndex}>
      {confirmDialog}
      <ul className="flex flex-col divide-y divide-carbon-border/60">
        {entries.map((entry) => {
          const { domain } = entry.item;
          const row = defaults.find((r) => r.domain === domain);
          const offered = options[domain];
          if (!row || !offered) return null;
          const uses = places(placeNames(t, entry.view, offered));
          const usual = places(placeNames(t, defaultView(row, offered), offered));
          return (
            <li key={`${domain}:${entry.item.key}`} className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3 first:pt-0 last:pb-0">
              <span
                aria-hidden="true"
                className="glim-hue inline-flex h-10 w-10 flex-none items-center justify-center rounded-control bg-carbon-surface2"
                style={hueVars(PLACEMENT_DOMAINS.indexOf(domain)) as CSSProperties}
              >
                <DomainGlyph domain={domain} />
              </span>
              <div className="flex min-w-0 flex-1 basis-48 flex-col gap-0.5">
                <Link
                  to={entryPath(entry.kind, entry.item.key)}
                  className="w-fit max-w-full truncate text-sm font-semibold text-carbon-text underline decoration-carbon-border underline-offset-[3px] hover:text-accentText"
                >
                  {entry.name}
                </Link>
                <span className="text-xs text-carbon-textMuted wrap-break-word">
                  {uses ? t("storage.choice.backsUpTo").replace("{places}", () => uses) : t("storage.choice.none")}
                  {usual && ` · ${t("storage.choice.default").replace("{places}", () => usual)}`}
                </span>
              </div>
              {followChange(entry.view) ? (
                <Button
                  label={t("storage.choice.follow")}
                  labelKey="storage.choice.follow"
                  glyph={<IconRefresh />}
                  tone="neutral"
                  disabled={busy}
                  onClick={() => void follow(entry, row, offered)}
                />
              ) : (
                <InfoBubble tip={lockHint(t, "home-fixed", viewHomeLabel(t, host, entry.view, offered))} />
              )}
            </li>
          );
        })}
      </ul>
    </Card>
  );
}
