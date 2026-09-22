import { useCallback, useState, type ReactNode } from "react";
import { ToggleRow } from "../../pages/settings/shared";
import {
  getNewTargetPreview,
  type NewTargetExclusion,
  type OffsiteDomain,
  type OkEnvelope,
  type TargetPreview,
} from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { formatList, isPlacementDomain } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { useConfirm } from "../../lib/useConfirm";

export interface NewTargetQuestion {
  domain: OffsiteDomain;
  location: string;
  targetId?: string;
  name: string;
  moved: boolean;
}

export type NewTargetAnswer = { go: false } | { go: true; alsoExclude: NewTargetExclusion | null };

const NOTHING: NewTargetExclusion = { identities: [], default: false };

/** itemName is the part of a snapshot name after its domain, "plex" for "container:plex". */
export function itemName(identity: string): string {
  return identity.slice(identity.indexOf(":") + 1);
}

/** NewTargetPreviewLines says what a target receives at its first run. With
 *  onExclusion it offers to leave out here what other targets leave out. */
export function NewTargetPreviewLines({
  target,
  preview,
  exclusion = NOTHING,
  onExclusion,
}: {
  target: string;
  preview: TargetPreview;
  exclusion?: NewTargetExclusion;
  onExclusion?: (next: NewTargetExclusion) => void;
}) {
  const { t, lang } = useT();
  const formerly = preview.formerlyExcluded.map((e) => itemName(e.identity));
  return (
    <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
      <p>{t("newTarget.items").replace("{n}", String(preview.items))}</p>
      <p>{t("newTarget.snapshots").replace("{n}", String(preview.snapshots))}</p>
      {preview.bytes !== null && <p>{t("newTarget.size").replace("{size}", humanBytes(preview.bytes))}</p>}
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
  target,
  preview,
  onChoice,
}: {
  target: string;
  preview: TargetPreview;
  onChoice: (next: NewTargetExclusion) => void;
}) {
  const [exclusion, setExclusion] = useState(NOTHING);
  return (
    <NewTargetPreviewLines
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

/** useNewTargetQuestion asks before a target starts receiving a domain's
 *  history. Flash and config have no items to leave out and are not asked. */
export function useNewTargetQuestion(): {
  ask: (q: NewTargetQuestion) => Promise<NewTargetAnswer>;
  dialog: ReactNode;
} {
  const { t, lang } = useT();
  const { confirm, confirmDialog } = useConfirm();

  const ask = useCallback(
    async (q: NewTargetQuestion): Promise<NewTargetAnswer> => {
      const domain = q.domain;
      if (!isPlacementDomain(domain)) return { go: true, alsoExclude: null };
      const intro = t(q.moved ? "newTarget.moved" : "newTarget.intro").replace("{target}", () => q.name);
      let res: OkEnvelope & { preview?: TargetPreview };
      try {
        res = await getNewTargetPreview(domain, q.location, q.targetId);
      } catch (err) {
        res = { ok: false, error: err instanceof Error ? err.message : undefined };
      }
      const preview = res.preview;
      if (!res.ok || !preview) {
        const reason = <p className="text-sm text-carbon-textSub">{placementErrorText(t, lang, res, "settings.error")}</p>;
        return (await confirm(intro, { extra: reason })) ? { go: true, alsoExclude: null } : { go: false };
      }
      let chosen = NOTHING;
      const extra = <ExclusionChoice target={q.name} preview={preview} onChoice={(next) => (chosen = next)} />;
      if (!(await confirm(intro, { extra }))) return { go: false };
      return { go: true, alsoExclude: chosen.identities.length > 0 || chosen.default ? chosen : null };
    },
    [confirm, t, lang]
  );

  return { ask, dialog: confirmDialog };
}
