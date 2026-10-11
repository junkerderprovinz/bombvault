import { useState } from "react";
import {
  confirmPlacementDefault,
  getConfirmPreview,
  type OkEnvelope,
  type PlacementDomain,
  type TargetPreviewRow,
  type UnmatchedName,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { ToggleRow } from "../../pages/settings/shared";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { IconCheck } from "../navGlyphs";
import { itemName, NewTargetPreviewLines } from "../placement/NewTargetQuestion";

type Preview = OkEnvelope & { targets?: TargetPreviewRow[]; unmatched?: UnmatchedName[] };

/** What the next run copies once the default is confirmed, and the names in
 *  the backups that have no entry here, each with a switch that leaves it out. */
function ConfirmLines({
  domain,
  targets,
  unmatched,
  onExclude,
}: {
  domain: PlacementDomain;
  targets: TargetPreviewRow[];
  unmatched: UnmatchedName[];
  onExclude: (identities: string[]) => void;
}) {
  const { t } = useT();
  const [excluded, setExcluded] = useState<string[]>([]);
  function toggle(identity: string, on: boolean) {
    const next = on ? [...excluded, identity] : excluded.filter((x) => x !== identity);
    setExcluded(next);
    onExclude(next);
  }
  return (
    <div className="flex flex-col gap-3">
      {targets.map((row) => (
        <div key={row.targetId} className="flex flex-col gap-1">
          <p className="text-sm text-carbon-text">{row.name}</p>
          <NewTargetPreviewLines domain={domain} target={row.name} preview={row.preview} />
        </div>
      ))}
      {unmatched.length > 0 && (
        <div className="flex flex-col gap-1">
          <p className="text-sm text-carbon-textSub">{t("placementDefaults.unmatched")}</p>
          {unmatched.map((u) => (
            <ToggleRow
              key={u.identity}
              label={t("placementDefaults.unmatchedLine")
                .replace("{name}", () => itemName(u.identity))
                .replace("{n}", String(u.snapshots))}
              checked={excluded.includes(u.identity)}
              onChange={(on) => toggle(u.identity, on)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * PausedDefault says that a section's off-site copies wait for its default to
 * be confirmed, which they do after a rebuild lost the rules, and confirms it
 * after showing what the next run would copy.
 */
export function PausedDefault({ domain }: { domain: PlacementDomain }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState(false);
  const label = t(DOMAIN_LABEL[domain]);
  const failed = (e: unknown): OkEnvelope => ({ ok: false, error: e instanceof Error ? e.message : undefined });

  async function confirmDefault() {
    setBusy(true);
    try {
      const preview: Preview = await getConfirmPreview(domain).catch(failed);
      if (!preview.ok) {
        push(placementErrorText(t, lang, preview, "settings.error"), "fail");
        return;
      }
      let excluded: string[] = [];
      const lines = (
        <ConfirmLines
          domain={domain}
          targets={preview.targets ?? []}
          unmatched={preview.unmatched ?? []}
          onExclude={(ids) => (excluded = ids)}
        />
      );
      const go = await confirm(t("placementDefaults.confirmAsk"), {
        extra: lines,
        confirmLabel: t("placementDefaults.confirm"),
        confirmLabelKey: "placementDefaults.confirm",
        cancelTone: "neutral",
      });
      if (!go) return;
      const res = await confirmPlacementDefault(domain, excluded).catch(failed);
      if (!res.ok) {
        push(placementErrorText(t, lang, res, "settings.error"), "fail");
        return;
      }
      push(t("placementDefaults.confirmed"), "success");
      placementChanged();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-wrap items-center justify-center gap-x-3 gap-y-2">
      {confirmDialog}
      <span className="flex items-center gap-1.5 text-xs text-statusWarn">
        {label}: {t("placementDefaults.paused")}
        <InfoBubble tip={t("placementDefaults.pausedHint").replace("{domain}", () => label)} />
      </span>
      <Button
        label={t("placementDefaults.confirm")}
        labelKey="placementDefaults.confirm"
        glyph={<IconCheck />}
        tone="neutral"
        disabled={busy}
        onClick={() => void confirmDefault()}
      />
    </div>
  );
}
