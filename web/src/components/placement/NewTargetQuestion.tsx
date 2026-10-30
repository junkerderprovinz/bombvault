import { useCallback, useState, type ReactNode } from "react";
import { ToggleRow } from "../../pages/settings/shared";
import {
  getNewTargetPreview,
  type NewLocationKind,
  type NewTargetExclusion,
  type OffsiteDomain,
  type TargetPreview,
} from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatList, isPlacementDomain } from "../../lib/placement";
import { placementErrorText, type PlacementRefusal } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";

export interface NewTargetQuestion {
  domain: OffsiteDomain;
  location: string;
  targetId?: string;
  name: string;
  moved: boolean;
  /** What the location becomes. With it, a location the save would refuse is
   *  refused before anything is asked. */
  as?: NewLocationKind;
}

/** refused says the location was turned down before the question opened, and
 *  the reason is already on screen. */
export type NewTargetAnswer = { go: false; refused?: boolean } | { go: true; alsoExclude: NewTargetExclusion | null };

const NOTHING: NewTargetExclusion = { identities: [], default: false };

/** itemName is the part of a snapshot name after its domain, "plex" for "container:plex". */
export function itemName(identity: string): string {
  return identity.slice(identity.indexOf(":") + 1);
}

/** NewTargetPreviewLines says what a target receives at its first run. With
 *  onExclusion it offers to leave out here what other targets leave out. */
export function NewTargetPreviewLines({
  domain,
  target,
  preview,
  exclusion = NOTHING,
  onExclusion,
}: {
  domain: OffsiteDomain;
  target: string;
  preview: TargetPreview;
  exclusion?: NewTargetExclusion;
  onExclusion?: (next: NewTargetExclusion) => void;
}) {
  const { t, lang } = useT();
  const formerly = preview.formerlyExcluded.map((e) => itemName(e.identity));
  const held = preview.held ?? 0;
  const sends = preview.snapshots - held;
  return (
    <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
      <p>{t(domain === "containers" ? "newTarget.itemsContainers" : "newTarget.items").replace("{n}", String(preview.items))}</p>
      {(sends > 0 || held === 0) && <p>{t("newTarget.snapshots").replace("{n}", String(sends))}</p>}
      {held > 0 && <p>{t("newTarget.held").replace("{n}", String(held))}</p>}
      {preview.bytes !== null && sends > 0 && <p>{t("newTarget.size").replace("{size}", humanBytes(preview.bytes))}</p>}
      {preview.unreadable.length > 0 && (
        <p>{t("placement.uncheckable").replace("{list}", () => formatList(lang, preview.unreadable))}</p>
      )}
      {formerly.length > 0 && <p>{t("newTarget.formerly").replace("{list}", () => formatList(lang, formerly))}</p>}
      {onExclusion && formerly.length > 0 && (
        <ToggleRow
          label={t("newTarget.excludeHere")}
          checked={exclusion.identities.length > 0}
          onChange={(on) =>
            onExclusion({ ...exclusion, identities: on ? preview.formerlyExcluded.map((e) => e.identity) : [] })
          }
        />
      )}
      {onExclusion && preview.defaultExcludes && (
        <ToggleRow
          label={t("newTarget.excludeDefault").replace("{target}", () => target)}
          checked={exclusion.default}
          onChange={(on) => onExclusion({ ...exclusion, default: on })}
        />
      )}
    </div>
  );
}

// Holds the switches while the question is open; the answer is read from
// onChoice once it closes.
function ExclusionChoice({
  domain,
  target,
  preview,
  onChoice,
}: {
  domain: OffsiteDomain;
  target: string;
  preview: TargetPreview;
  onChoice: (next: NewTargetExclusion) => void;
}) {
  const [exclusion, setExclusion] = useState(NOTHING);
  return (
    <NewTargetPreviewLines
      domain={domain}
      target={target}
      preview={preview}
      exclusion={exclusion}
      onExclusion={(next) => {
        setExclusion(next);
        onChoice(next);
      }}
    />
  );
}

// A move to another route to the same storage sends nothing again, so the
// question opens with what the new location holds.
function introKey(moved: boolean, preview?: TargetPreview): TranslationKey {
  if (!moved) return "newTarget.intro";
  if (!preview || preview.held === null || preview.snapshots === 0) return "newTarget.moved";
  if (preview.held === 0) return "newTarget.movedEmpty";
  return preview.held < preview.snapshots ? "newTarget.movedPartly" : "newTarget.movedHeld";
}

/** useNewTargetQuestion asks before a target starts receiving a domain's
 *  history. Flash and config have no items to leave out and are not asked. */
export function useNewTargetQuestion(): {
  ask: (q: NewTargetQuestion) => Promise<NewTargetAnswer>;
  dialog: ReactNode;
} {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();

  const ask = useCallback(
    async (q: NewTargetQuestion): Promise<NewTargetAnswer> => {
      const domain = q.domain;
      if (!isPlacementDomain(domain)) return { go: true, alsoExclude: null };
      let res: PlacementRefusal & { preview?: TargetPreview; saveRefused?: boolean };
      try {
        res = await getNewTargetPreview(domain, q.location, q.targetId, q.as);
      } catch (err) {
        res = { ok: false, error: err instanceof Error ? err.message : undefined };
      }
      if (res.saveRefused) {
        push(placementErrorText(t, lang, res, "settings.error"), "fail");
        return { go: false, refused: true };
      }
      const preview = res.preview;
      const intro = t(introKey(q.moved, preview)).replace("{target}", () => q.name);
      if (!res.ok || !preview) {
        const reason = <p className="text-sm text-carbon-textSub">{placementErrorText(t, lang, res, "settings.error")}</p>;
        return (await confirm(intro, { extra: reason, cancelTone: "neutral" })) ? { go: true, alsoExclude: null } : { go: false };
      }
      let chosen = NOTHING;
      const extra = <ExclusionChoice domain={domain} target={q.name} preview={preview} onChoice={(next) => (chosen = next)} />;
      if (!(await confirm(intro, { extra, cancelTone: "neutral" }))) return { go: false };
      return { go: true, alsoExclude: chosen.identities.length > 0 || chosen.default ? chosen : null };
    },
    [confirm, push, t, lang]
  );

  return { ask, dialog: confirmDialog };
}
